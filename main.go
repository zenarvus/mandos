package main
import ("log"; "mime"; "path"; "path/filepath"; "github.com/gofiber/fiber/v2"; "github.com/gofiber/fiber/v2/middleware/compress")

func main() {
	loadTemplates("md"); loadTemplates("solo"); loadNotesAndAttachments(); go watchFileChanges()
	app := fiber.New(); initRoutes(app)
	certFile:=getEnvValue("CERT"); keyFile:=getEnvValue("KEY")
	if certFile=="" || keyFile=="" {err := app.Listen(":"+getEnvValue("PORT")); if err!=nil{panic(err)}
	}else{err := app.ListenTLS(":"+getEnvValue("PORT"),certFile,keyFile); if err!=nil{panic(err)}}
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
	//Only markdown files with public: true metadata and their previewed attachments are served
	app.Get("/*", func(c *fiber.Ctx) error {
		urlPath := "/"+c.Params("*"); if urlPath=="/"{urlPath += indexPage}; ext := filepath.Ext(urlPath)

		switch ext {
		// If the wanted file is markdown, parse the template and serve if its served.
		case ".md":
			var fileinfo Node
			if wNode:=servedNodes[urlPath]; wNode!=nil && wNode.File!=""{ fileinfo, _ = getFileInfo(path.Join(notesPath,urlPath),true) }

			if fileinfo.Content == "" && fileinfo.Title == "" {
				fileinfo.Content = "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>"
				fileinfo.Title = "404 Not Found"
			}
			c.Response().Header.Add("Content-Type", "text/html")

			templateName,ok := fileinfo.Params["template"].(string)
			if !ok || templateName == "" {templateName = "main.html"}
			// Render the template
			if mdTemplates[templateName] != nil {
				err := mdTemplates[templateName].Execute(c.Response().BodyWriter(), fileinfo)
				if err!=nil {log.Println(err)}; return nil
			}else{return c.SendString("No template found")}
		// If the wanted file is not markdown
		default:
			if servedAttachments[urlPath] {
				// If the file is a solo template
				if soloTemplates[urlPath] != nil{
					c.Response().Header.Add("Content-Type",mime.TypeByExtension(filepath.Ext(urlPath)))
					err := soloTemplates[urlPath].Execute(c.Response().BodyWriter(), map[string]string{})
					if err!=nil {log.Println(err)}; return nil
				}
				// Else, send the file directly.
				c.Response().Header.Add("Cache-Control", "max-age=604800")
				return c.SendFile(path.Join(notesPath, urlPath))
			}else{return c.SendStatus(404)}
		}
	})
}
