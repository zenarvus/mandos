package main

import (
	"path"; "path/filepath";
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
)

func main() {
	loadMdTemplates(); loadNotesAndAttachments(); go watchFileChanges()
	app := fiber.New(); initRoutes(app)
	certFile:=getEnvValue("CERT"); keyFile:=getEnvValue("KEY")
	if certFile=="" || keyFile=="" {
		err := app.Listen(":"+getEnvValue("PORT")); if err!=nil{panic(err)}
	} else {
		err := app.ListenTLS(":"+getEnvValue("PORT"),certFile,keyFile); if err!=nil{panic(err)}
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

	//Only markdown files with public: true metadata and their previewed attachments are served
	app.Get("/*", func(c *fiber.Ctx) error {
		urlPath := "/"+c.Params("*")
		if urlPath=="/"{urlPath+=indexPage}; ext := filepath.Ext(urlPath)

		switch ext {
		// If the wanted file is markdown, parse the template and serve if its served.
		case ".md":
			var fileinfo Node
			if servedNodes[urlPath].File != "" { fileinfo, _ = getFileInfo(path.Join(notesPath,urlPath),true) }

			if fileinfo.Content == "" && fileinfo.Title == "" {
				fileinfo.Content = "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>"
				fileinfo.Title = "404 Not Found"
			}

			templateValues := map[string]any{
				"Metadata": fileinfo.Metadata,
				"Content": fileinfo.Content,
				"File": fileinfo.File,
				"Title": fileinfo.Title,
				"OutLinks": fileinfo.OutLinks,
				"InLinks": fileinfo.InLinks,
			}
			c.Response().Header.Add("Content-Type", "text/html")

			templateName := fileinfo.Metadata["template"]
			if templateName == "" {templateName = "main.html"}
			// Render the template
			if mdTemplates[templateName] != nil {
				err := mdTemplates[templateName].Execute(c.Response().BodyWriter(), templateValues)
				if err != nil {return c.Status(500).SendString("An error occurred while executing the template: "+err.Error())}
				return nil
			}else{return c.SendString("No template found")}
		
		// If the wanted file is not markdown
		default:
			if servedAttachments[urlPath] {
				// If its an XML or JSON file, execute the template.
				if ext==".xml" || ext==".json"{
					if ext==".xml"{c.Response().Header.Add("Content-Type","application/xml")
					}else if ext==".json"{c.Response().Header.Add("Content-Type","application/json")}
					tmpl,err := readTemplateFile(path.Join(notesPath, urlPath), urlPath)
					if err!=nil{return c.Status(500).SendString("An error occurred while parsing the template: "+err.Error())}
					err = tmpl.Execute(c.Response().BodyWriter(),map[string]any{})
					if err != nil {return c.Status(500).SendString("An error occurred while executing the template: "+err.Error())}
					return nil
				}
				// Else, send the file directly.
				c.Response().Header.Add("Cache-Control", "max-age=604800")
				return c.SendFile(path.Join(notesPath, urlPath))
			}else{return c.SendStatus(404)}
		}
	})
}
