package main

import (
	"database/sql"
	"fmt"
	"log"
	"mime"
	"path"
	"path/filepath"
	"runtime"
	"time"

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

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		// Protective Timeouts
		ReadTimeout:  5 * time.Second,  // Time allowed to read the full request body
		WriteTimeout: 10 * time.Second, // Time allowed to write the response
		IdleTimeout:  120 * time.Second, // Time a keep-alive connection stays open
	})

	initRoutes(app)

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
		urlPath := "/"+c.Params("*");
		if urlPath=="/"{urlPath += indexPage};

		switch filepath.Ext(urlPath) {
		// If the wanted file is markdown, parse the template and serve if its served.
		case ".md":
			// Prefer the cached node
			nodeInfo,exists := nodeCache.Get(urlPath)
			if !exists {
				nodeInfo,_ = getNodeInfo(urlPath, false);
				nodeCache.Put(urlPath, nodeInfo) // Add node to the cache.
			}

			// If the node is not public or has no content.
			if !isServed(nodeInfo.Public) || nodeInfo.Content == "" {
				if mdTemplates["/mandos/404.html"] != nil {
					err := mdTemplates["/mandos/404.html"].Execute(c.Response().BodyWriter(), PageVars{Url: c.BaseURL()+c.OriginalURL(), Node: &nodeInfo})
					if err!=nil {fmt.Println(err)}; return nil
				} else {
					nodeInfo = Node{
						Title: "404 Not Found", Content: "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>",
					}
				}
			}

			c.Response().Header.Add("Content-Type", "text/html")

			templateName,ok := nodeInfo.Params["template"].(string)
			if !ok || templateName == "" {templateName = "main.html"}
			templateRelPath := filepath.Join("/mandos/",templateName)

			// Render the template
			if mdTemplates[templateRelPath] != nil {
				err := mdTemplates[templateRelPath].Execute(c.Response().BodyWriter(), PageVars{Url: c.BaseURL()+c.OriginalURL(), Node: &nodeInfo})
				if err!=nil {fmt.Println(err)}; return nil

			}else{return c.SendString("No template found")}

		// If the wanted file is not markdown
		default:
			// Prefer the cached attachment check.
			attachment, exists := attachmentCache.Get(urlPath)
			if !exists {
				// Check if at least one node has a link to the attachment.
				err := DB.QueryRow(`SELECT file FROM attachments WHERE "file" = ? LIMIT 1;`, urlPath).Scan(&attachment)
				if err != nil {
					if err == sql.ErrNoRows { return c.SendStatus(404) }
					log.Println("Database error:", err); return c.SendStatus(500)
				}
				attachmentCache.Set(urlPath, urlPath, time.Second*30) // Save to the cache.
			}

			// If we reach here, the attachment is found.
			if soloTemplates[urlPath] != nil{ // If the file is a solo template
				c.Response().Header.Add("Content-Type",mime.TypeByExtension(filepath.Ext(urlPath)))
				err := soloTemplates[urlPath].Execute(c.Response().BodyWriter(), PageVars{Url: c.BaseURL()+c.OriginalURL()})
				if err!=nil {fmt.Println(err)}; return nil
			}
			// Else, send the file directly.
			c.Response().Header.Add("Cache-Control", "max-age=604800")
			return c.SendFile(path.Join(notesPath, urlPath))
		}
	})
}
