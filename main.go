package main

import (
	"github.com/gofiber/fiber/v2"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

func main() {
	loadTemplates()
	loadNotesAndAttachments()
	go watchFileChanges()

	app := fiber.New()
	initRoutes(app)

	certFile:=getEnvValue("CERT")
	keyFile:=getEnvValue("KEY")
	if certFile=="" || keyFile=="" {
		err := app.Listen(":"+getEnvValue("PORT"))
		if err!=nil{panic(err)}
	} else {
		err := app.ListenTLS(":"+getEnvValue("PORT"),certFile,keyFile)
		if err!=nil{panic(err)}
	}
}

func initRoutes(app *fiber.App){
	//All files in static folder are served
	app.Static("/static", path.Join(notesPath,"/static"), fiber.Static{MaxAge:60*60*24*7})

	// Compress with gzip if its ends with css, js, txt or md. Skip compression if its not them
	app.Use(compress.New(compress.Config{
	  Next:  func(c *fiber.Ctx) bool {
		ext:=filepath.Ext(c.Path())
		return ext != ".md" && ext != ".js" && ext != ".css" && ext != ".txt"
	  },
	  Level: compress.LevelBestSpeed, // 1
	}))

	//Send a node list data to the client
	app.Get("/node-list", func(c *fiber.Ctx)error{
		var nodeArr []map[string]any
		for _,servedFile := range servedFiles {
			nodeArr = append(nodeArr, map[string]any{
				"title": servedFile.Title,
				"file": servedFile.MapKey,
				"outlinks": servedFile.OutLinks,
				"inlinks": servedFile.InLinks,
			})
		}
		return c.JSON(nodeArr)
	})

	app.Get("/rss", func(c *fiber.Ctx)error{
		var timeAwareNodes []servedFile
		for _, fileInfo := range servedFiles {
			if fileInfo.Metadata["date"] != "" { timeAwareNodes = append(timeAwareNodes, fileInfo) }
		}
		sort.Sort(sortNodesByDate(timeAwareNodes))
		return c.XML(ConvertToRSS(timeAwareNodes, strings.TrimSuffix(c.BaseURL(),"/"), c.Hostname()))
	})

	//Only markdown files with public: true metadata and their previewed attachments are served
	app.Get("/*", func(c *fiber.Ctx) error {
		urlPath := "/"+c.Params("*")
		if urlPath=="/"{urlPath+=indexPage}; ext := filepath.Ext(urlPath)

		switch ext {
		// If the wanted file is markdown, parse the template and serve if its served.
		case ".md", ".mdx":
			var fileinfo fileInfo
			if servedFiles[urlPath].MapKey != "" { fileinfo, _ = getFileInfo(path.Join(notesPath,urlPath),true) }

			if fileinfo.Content == "" && fileinfo.Title == "" {
				fileinfo.Content = "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>"
				fileinfo.Title = "404 Not Found"
			}

			templateValues := map[string]any{
				"Metadata": fileinfo.Metadata,
				"Content": fileinfo.Content,
				"File": strings.TrimPrefix(urlPath,"/"),
				"Title": fileinfo.Title,
			}
			c.Response().Header.Add("Content-Type", "text/html")

			templateName := fileinfo.Metadata["template"]
			if templateName == "" {templateName = "main.html"}
			// Render the template
			if templates[templateName] != nil {
				err := templates[templateName].Execute(c.Response().BodyWriter(), templateValues)
				if err != nil {return c.Status(500).SendString("Error executing template")}
				return nil
			}else{return c.SendString("No template found")}
		
		// If the wanted file is not markdown
		default:
			if servedFiles[urlPath].MapKey != ""{
				c.Response().Header.Add("Cache-Control", "max-age=604800")
				return c.SendFile(path.Join(notesPath, urlPath))
			}else{return c.SendStatus(404)}
		}
	})
}
