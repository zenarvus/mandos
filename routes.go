package main

import (
	"path"
	"strings"
	"bytes"
	"github.com/gofiber/fiber/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
)

var htmlConverter = goldmark.New(
    goldmark.WithExtensions(extension.GFM, extension.Footnote),
    goldmark.WithParserOptions(parser.WithAttribute()),
    goldmark.WithRendererOptions(
        goldmarkHtml.WithHardWraps(),
        goldmarkHtml.WithXHTML(),
        goldmarkHtml.WithUnsafe(),
    ),
)

func initRoutes(app *fiber.App){
    //All files in static folder are served
    app.Static("/static", path.Join(notesPath,"/static"))

	//Send all nodes to the client
	app.Get("/node-list", func(c *fiber.Ctx)error{
		return c.JSON(servedFiles)
	})

    //Only markdown files with <!--public--> metadata and their previewed attachments are served
    app.Get("/*", func(c *fiber.Ctx) error {
        urlPath := c.Path() //strings.TrimSuffix(c.Path(),"/")

        if !strings.Contains(urlPath,"/media/"){
            if urlPath=="/"{urlPath="/"+indexPage}

			var fileInfo fileInfo
            if servedFiles[urlPath] != ""{
				fileInfo, _ = getFileInfo(path.Join(notesPath,urlPath))
            }

			if fileInfo.Content == "" && fileInfo.Title == "" {
				fileInfo.Content = "<p style='text-align:center;'>404 file does not exist</p>"
				fileInfo.Title = "404 Not Found"
			}

			//convert md content to html
			var html bytes.Buffer
			if err := htmlConverter.Convert([]byte(fileInfo.Content), &html);
			err != nil {panic(err)}

			var nodeAuthor string
			if fileInfo.Metadata["author"] != "" {
				nodeAuthor = fileInfo.Metadata["author"]
			}else{nodeAuthor = author}

			templateValues := map[string]interface{}{
                "Host": strings.Split(c.Hostname(),".")[0],
                "Content": html.String(),
                "File": strings.TrimPrefix(urlPath,"/"),
                "Title": fileInfo.Title,
                "Author": nodeAuthor,
            }

            c.Response().Header.Add("Content-Type", "text/html")

			templateName := fileInfo.Metadata["template"]
			if templateName == "" {templateName = "main.html"}
			// Render the template
			if templates[templateName] != nil {
				err := templates[templateName].Execute(c.Response().BodyWriter(), templateValues)
				if err != nil {return c.Status(500).SendString("Error executing template")}
				return nil
			}else{return c.SendString("No template found")}

        } else {
          if servedFiles[urlPath] != ""{
            return c.SendFile(path.Join(notesPath,urlPath))
          }else{
            return c.SendStatus(404)
          }
        }
    })
}
