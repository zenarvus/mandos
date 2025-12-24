package main

import (
	"fmt"
	"log"
	"mime"
	"path"
	"path/filepath"
	"runtime"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

func main() {
	InitDB(); defer DB.Close()

	fmt.Println("Folder:",notesPath); fmt.Println("Index:", indexPage)

	loadAllTemplates("md"); loadAllTemplates("solo");

	initialSyncWithDB()

	var servedNodes int
	rows, _ := DB.Query(`SELECT COUNT(*) AS row_count FROM nodes;`)
	for rows.Next() {rows.Scan(&servedNodes)}
	fmt.Println("Nodes Served:", servedNodes)

	go watchFileChanges()

	app := fiber.New(fiber.Config{DisableStartupMessage:true}); initRoutes(app)

	var m runtime.MemStats; runtime.ReadMemStats(&m)
	fmt.Printf("Memory Used: %.2f MiB\n", float64(m.Sys)/1024/1024)

	fmt.Println("Server is started on port", getEnvValue("PORT"))
	certFile:=getEnvValue("CERT"); keyFile:=getEnvValue("KEY")
	if certFile=="" || keyFile=="" {
		err := app.Listen(":"+getEnvValue("PORT")); if err!=nil{panic(err)}
	}else{
		fmt.Println("TLS Mode Active")
		err := app.ListenTLS(":"+getEnvValue("PORT"),certFile,keyFile); if err!=nil{panic(err)}
	}
}
func initRoutes(app *fiber.App){
	//All files in static folder are served
	app.Static("/static", path.Join(notesPath,"/static"), fiber.Static{MaxAge:60*60*24*7})
	// Compress with gzip if its ends with ,css, .html, .json, js, .xml, txt or md. Skip compression if its not them
	app.Use(compress.New(compress.Config{
		Next:  func(c *fiber.Ctx) bool {
			ext:=filepath.Ext(c.Path())
			return ext != ".md" && ext != ".js" && ext != ".css" && ext != ".txt" && ext != ".json" && ext != ".xml" && ext != ".html"
		},
		Level: compress.LevelBestSpeed, // 1
	}))

	type PageVars struct { Url string; *Node }

	//Only markdown files with public: true metadata and their previewed attachments are served
	app.Get("/*", func(c *fiber.Ctx) error {
		urlPath := "/"+c.Params("*"); if urlPath=="/"{urlPath += indexPage}; ext := filepath.Ext(urlPath)

		switch ext {
		// If the wanted file is markdown, parse the template and serve if its served.
		case ".md":
			var nodeinfo,_ = getNodeInfo(urlPath)
			if !isServed(nodeinfo.Public) || nodeinfo.Content == ""{
				if mdTemplates["/mandos/404.html"] != nil {
					err := mdTemplates["/mandos/404.html"].Execute(c.Response().BodyWriter(), PageVars{Url: c.BaseURL()+c.OriginalURL(), Node: &nodeinfo})
					if err!=nil {fmt.Println(err)}; return nil

				} else {
					nodeinfo = Node{
						Content: "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>",
						Title: "404 Not Found",
					}
				}
			}
			nodeinfo.File = &urlPath

			c.Response().Header.Add("Content-Type", "text/html")

			templateName,ok := nodeinfo.Params["template"].(string)
			if !ok || templateName == "" {templateName = "main.html"}
			templateRelPath := filepath.Join("/mandos/",templateName)
			// Render the template
			if mdTemplates[templateRelPath] != nil {
				err := mdTemplates[templateRelPath].Execute(c.Response().BodyWriter(), PageVars{Url: c.BaseURL()+c.OriginalURL(), Node: &nodeinfo})
				if err!=nil {fmt.Println(err)}; return nil
			}else{return c.SendString("No template found")}
		// If the wanted file is not markdown
		default:
			var attachment string

			rows, err := DB.Query(`SELECT file FROM attachments WHERE "file" = ? LIMIT 1;`, urlPath)
			if err != nil { log.Println(err) }
			defer rows.Close()
			for rows.Next() { if err := rows.Scan(&attachment); err != nil { log.Println(err) } }
			if err!=nil{log.Fatalln(err)}

			if attachment != "" {
				// If the file is a solo template
				if soloTemplates[urlPath] != nil{
					c.Response().Header.Add("Content-Type",mime.TypeByExtension(filepath.Ext(urlPath)))
					err := soloTemplates[urlPath].Execute(c.Response().BodyWriter(), PageVars{Url: c.BaseURL()+c.OriginalURL()})
					if err!=nil {fmt.Println(err)}; return nil
				}
				// Else, send the file directly.
				c.Response().Header.Add("Cache-Control", "max-age=604800")
				return c.SendFile(path.Join(notesPath, urlPath))

			}else{return c.SendStatus(404)}
		}
	})
}
