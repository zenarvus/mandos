package main

import (
	"bytes"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"github.com/gofiber/fiber/v2"
	mathjax "github.com/litao91/goldmark-mathjax"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"

	"github.com/mdigger/goldmark-attributes"
)

var htmlConverter = goldmark.New(
	attributes.Enable,
	goldmark.WithExtensions(extension.GFM, extension.Footnote, mathjax.MathJax, BetterMediaExt()),
	goldmark.WithParserOptions(parser.WithAttribute()),
	goldmark.WithRendererOptions(
		goldmarkHtml.WithHardWraps(),
		goldmarkHtml.WithXHTML(),
		goldmarkHtml.WithUnsafe(),
	),
)

func initRoutes(app *fiber.App){
	//All files in static folder are served
	app.Static("/static", path.Join(notesPath,"/static"), fiber.Static{MaxAge:60*60*24*7})

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
			if fileInfo.Date != "" {
				timeAwareNodes = append(timeAwareNodes, fileInfo)
			}
		}
		sort.Sort(sortNodesByDate(timeAwareNodes))

		return c.XML(ConvertToRSS(timeAwareNodes, strings.TrimSuffix(c.BaseURL(),"/"), c.Hostname()))
	})

	//Only markdown files with <!--public--> metadata and their previewed attachments are served
	app.Get("/*", func(c *fiber.Ctx) error {
		urlPath := "/"+c.Params("*")
		if urlPath=="/"{urlPath+=indexPage}

		// If the wanted file is markdown
		if filepath.Ext(urlPath)==".md" || filepath.Ext(urlPath)==".mdx"{
			var fileinfo fileInfo
			if servedFiles[urlPath].MapKey != "" {
				fileinfo, _ = getFileInfo(path.Join(notesPath,urlPath),true)
			}

			if fileinfo.Content == "" && fileinfo.Title == "" {
				fileinfo.Content = "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>"
				fileinfo.Title = "404 Not Found"
			}

			//convert md content to html
			var html bytes.Buffer
			if err := htmlConverter.Convert([]byte(fileinfo.Content), &html);
			err != nil {panic(err)}

			var nodeAuthor string
			if fileinfo.Metadata["author"] != "" {
				nodeAuthor = fileinfo.Metadata["author"]
			}else{nodeAuthor = author}

			templateValues := map[string]any{
				"Host": c.Hostname(),
				"Metadata": fileinfo.Metadata,
				"Content": html.String(),
				"File": strings.TrimPrefix(urlPath,"/"),
				"Title": fileinfo.Title,
				"Author": nodeAuthor,
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
		}else{
			if servedFiles[urlPath].MapKey != ""{
				c.Response().Header.Add("Cache-Control", "max-age=604800")
				return c.SendFile(path.Join(notesPath, urlPath))
			}else{return c.SendStatus(404)}
		}
	})
}
