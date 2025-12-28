package main

import (
	"database/sql"
	"fmt"
	"log"
	"mime"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	"github.com/gofiber/fiber/v2/middleware/limiter"
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

	fiberConfig := fiber.Config{
		DisableStartupMessage: true,
		// Protective Timeouts
		ReadTimeout:  5 * time.Second,  // Time allowed to read the full request body
		WriteTimeout: 10 * time.Second, // Time allowed to write the response
		IdleTimeout:  120 * time.Second, // Time a keep-alive connection stays open
	}

	trustedProxiesStr := getEnvValue("TRUSTED_PROXIES")
	if trustedProxiesStr != "" {
		proxies := strings.Split(trustedProxiesStr, ",")
		fiberConfig.EnableTrustedProxyCheck = true
		fiberConfig.TrustedProxies = proxies
	}

	app := fiber.New(fiberConfig)

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
	// Set the rate limits for markdown and attachments, also set the limit values for solo templates.
	rateLimitStr := getEnvValue("RATE_LIMIT")
	var limits []string
	if rateLimitStr != "" { limits = strings.Split(rateLimitStr, ",") }
	var soloLimits = make(map[string][]int)
	for _,limit := range limits {
		parts := strings.Split(limit, ":")
		if len(parts) != 3 {log.Fatalln("Malformed rate limit setting:", limit)}

		// Implement rate limiting to markdown files.
		if parts[0] == "!md" {
			app.Use(limiter.New(limiter.Config{
				Next: func(c *fiber.Ctx) bool {
					// If it's a markdown file, do not skip it, use the rate limiter. Else, skip.
					return !strings.HasSuffix(c.Path(), ".md")
				},
				Expiration: time.Duration(convertToInt(parts[1])) * time.Second,
				Max: convertToInt(parts[2]),
				KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
			}))
			log.Println("Rate limit is applied for markdown files.")
		
		// Implement rate limiting to attachments.
		} else if parts[0] == "!att" {
			app.Use(limiter.New(limiter.Config{
				Next: func(c *fiber.Ctx) bool {
					// If it's not a markdown file nor solo template, apply the attachment rate limiter. Else, skip.
					return strings.HasSuffix(c.Path(), ".md") || soloTemplates[c.Path()] != nil
				},
				Expiration: time.Duration(convertToInt(parts[1])) * time.Second,
				Max: convertToInt(parts[2]),
				KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
			}))
			log.Println("Rate limit is applied for markdown attachments and static files.")

		// Save solo template limit values.
		} else {
			soloLimits[filepath.Join("/",parts[0])] = []int{convertToInt(parts[1]), convertToInt(parts[2])}
		}

	}

	// All files in static folder are served
	app.Static("/static", path.Join(notesPath,"/static"), fiber.Static{MaxAge:60*60*24*7})
	// Compress with gzip if its ends with ,css, .html, .json, js, .xml, txt or md. Skip compression if its not them
	app.Use(compress.New(compress.Config{
		Next:  func(c *fiber.Ctx) bool {
			ext:=filepath.Ext(c.Path())
			return ext != ".md" && ext != ".js" && ext != ".css" && ext != ".txt" && ext != ".json" && ext != ".xml" && ext != ".html"
		},
		Level: compress.LevelBestSpeed, // 1
	}))

	type PageVars struct { AccessTime int64; Url string; *Node; Headers map[string]string; Form map[string]string; }

	// Serve the solo templates.
	for soloPath := range soloTemplates {

		fiberHander := func(c *fiber.Ctx) error {
			// If the file is a solo template and have permission to handle with POST requests, execute it.
			if soloTemplates[c.Path()] != nil{
				var headers = make(map[string]string)
				for header,values := range c.GetReqHeaders() { headers[header] = values[0] }

				contentType := mime.TypeByExtension(filepath.Ext(c.Path()))
				if contentType == "" {contentType = "text/plain"}

				pagevars := PageVars{ Url:c.BaseURL()+c.OriginalURL(), Headers:headers, AccessTime: time.Now().Unix() }
				if c.Method() == "POST" {
					var formData = make(map[string]string)
					form,_ := c.MultipartForm()
					if form != nil {
						for key, values := range form.Value { formData[key] = values[0] }
					}
					pagevars.Form = formData
				}

				c.Response().Header.Add("Content-Type", contentType)
				err := soloTemplates[soloPath].Execute(c.Response().BodyWriter(), pagevars)
				if err!=nil {fmt.Println(err)};
				return nil;
			}
			// Else, return not found error.
			return c.SendStatus(404)
		}

		if len(soloLimits[soloPath]) == 2 {
			expr := soloLimits[soloPath][0]; maximum := soloLimits[soloPath][1]
			limit := limiter.New(limiter.Config{Expiration:time.Duration(expr)*time.Second, Max:maximum})
			app.Get(soloPath, limit, fiberHander)
			app.Post(soloPath, limit, fiberHander)
			log.Println("Rate limit is applied for:", soloPath)
			delete(soloLimits, soloPath)
		} else { app.Get(soloPath, fiberHander); app.Post(soloPath, fiberHander) }
	}

	// If any soloLimits element is left. It means that solo template for it does not exists.
	for soloLimit := range soloLimits {log.Println("Solo template for the limit does not exists:",soloLimit)}

	// Only markdown files with public: true metadata and their previewed attachments are served
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

			var headers = make(map[string]string)
			for header,values := range c.GetReqHeaders() { headers[header] = values[0] }

			// If the node is not public or has no content.
			if !isServed(nodeInfo.Public) || nodeInfo.Content == "" {
				if mdTemplates["/mandos/404.html"] != nil {
					err := mdTemplates["/mandos/404.html"].Execute(c.Response().BodyWriter(), PageVars{
						Url: c.BaseURL()+c.OriginalURL(), Node: &nodeInfo, Headers: headers, AccessTime: time.Now().Unix(),
					})
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
			templateRelPath := strings.TrimPrefix(filepath.Join(getEnvValue("MD_TEMPLATES"),templateName), notesPath)

			// Render the template
			if mdTemplates[templateRelPath] != nil {
				err := mdTemplates[templateRelPath].Execute(c.Response().BodyWriter(), PageVars{
					Url: c.BaseURL()+c.OriginalURL(), Node: &nodeInfo, Headers: headers,
				})
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
			c.Response().Header.Add("Cache-Control", "max-age=604800")
			return c.SendFile(path.Join(notesPath, urlPath))
		}
	})
}
