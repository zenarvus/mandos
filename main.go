package main

import (
	"database/sql"; "fmt"; "log"; "mime"; "path"; "path/filepath"; "runtime"
	"strings"; "time"; "bytes"

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
	DB.QueryRow(`SELECT COUNT(*) FROM nodes`).Scan(&servedNodes)
	fmt.Println("Nodes Served:", servedNodes)

	go watchFileChanges()

	fiberConfig := fiber.Config{
		DisableStartupMessage: true,
		// Protective Timeouts
		ReadTimeout:  5 * time.Second,  // Time allowed to read the full request body
		WriteTimeout: 10 * time.Second, // Time allowed to write the response
		IdleTimeout:  120 * time.Second, // Time a keep-alive connection stays open
	}

	behindProxy := getEnvValue("BEHIND_PROXY")
	if behindProxy=="true" { fiberConfig.ProxyHeader = "X-Forwarded-For" }

	app := fiber.New(fiberConfig)

	attExistStmt := initRoutes(app)
	if attExistStmt != nil { defer attExistStmt.Close() }

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
func initRoutes(app *fiber.App) *sql.Stmt {

	noAttCheck := getEnvValue("NO_ATTACHMENT_CHECK") == "true"
	// Prepare the attachment existence check statement.
	attExistStmt,_ := DB.Prepare(`SELECT file FROM attachments WHERE "file" = ? LIMIT 1;`)

	if getEnvValue("LOGGING")=="true" {
		app.Use(func(c *fiber.Ctx)error{ log.Println(c.IP(), c.Path()); return c.Next() })
	}

	// Set the rate limits for markdown and attachments, also set the limit values for solo templates.
	rateLimitStr := getEnvValue("RATE_LIMIT")
	var limits []string
	if rateLimitStr != "" { limits = strings.Split(rateLimitStr, ",") }

	var soloLimits = make(map[string][]int)

	for _,limit := range limits {
		parts := strings.Split(limit, ":")
		if len(parts) != 3 {log.Fatalln("Malformed rate limit setting:", limit)}

		var limitSkipFuncs = map[string]func(path string)bool{
			// If it is not a markdown file, skip the limiter middleware. Else, use it.
			"!md": func(path string)bool{ return !strings.HasSuffix(path, ".md") },
			// If it is not a markdown file or a solo template, skip the limiter middleware. Else, use it.
			"!att": func(path string)bool{ return strings.HasSuffix(path, ".md") || soloTemplates[path] != nil },
		}
		// Implement rate limiting for markdown files and attachments.
		if parts[0] == "!md" || parts[0] == "!att" {
			app.Use(limiter.New(limiter.Config{
				Next: func(c *fiber.Ctx) bool {
					return limitSkipFuncs[parts[0]](c.Path())
				},
				Expiration: time.Duration(convertToInt(parts[1])) * time.Second,
				Max: convertToInt(parts[2]),
				KeyGenerator: func(c *fiber.Ctx) string { return c.IP() },
			}))
			log.Println("Rate limit is applied for", parts[0])
		
		// If its a solo template rate limit, save limit values to use while generating solo template endpoints.
		} else {
			soloLimits[filepath.Join("/",parts[0])] = []int{convertToInt(parts[1]), convertToInt(parts[2])}
		}

	}

	// Compress with gzip if its ends with ,css, .html, .json, js, .xml, txt or md. Skip compression if its not them
	var compressed = map[string]bool{".md":true, ".js":true, ".css":true, ".txt":true, ".json":true, ".xml":true, ".html":true}
	app.Use(compress.New(compress.Config{
		Next:  func(c *fiber.Ctx) bool { return !compressed[ filepath.Ext(c.Path()) ] },
		Level: compress.LevelBestSpeed, // 1
	}))

	// All files in static folder are served
	app.Static("/static", path.Join(notesPath,"/static"), fiber.Static{MaxAge:60*60*24*7})

	type PageVars struct { Now int64; Url string; *Node; Ctx *fiber.Ctx; }

	// Serve the solo templates.
	for soloPath := range soloTemplates {

		fiberHander := func(c *fiber.Ctx) error {
			// If the file is a solo template and have permission to handle with POST requests, execute it.
			if soloTemplates[c.Path()] != nil{

				contentType := mime.TypeByExtension(filepath.Ext(c.Path()))
				if contentType == "" {contentType = "text/plain"}

				pagevars := PageVars{ Url:c.BaseURL()+c.OriginalURL(), Ctx: c, Now: time.Now().Unix() }

				buf := new(bytes.Buffer)
				c.Response().Header.Add("Content-Type", contentType)

				err := soloTemplates[soloPath].Execute(buf, pagevars)
				if err!=nil {fmt.Println(err); return c.Status(500).SendString(err.Error())};

				return c.Send(buf.Bytes());
			}
			// Else, return not found error.
			return c.SendStatus(404)
		}

		if len(soloLimits[soloPath]) == 2 {
			expr := soloLimits[soloPath][0]; maximum := soloLimits[soloPath][1]
			limit := limiter.New(limiter.Config{Expiration:time.Duration(expr)*time.Second, Max:maximum})
			app.Get(soloPath, limit, fiberHander); app.Post(soloPath, limit, fiberHander)

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

			// If the node is not public or has no content.
			if !isServed(nodeInfo.Public) || nodeInfo.Content == "" {
				if mdTemplates["/mandos/404.html"] != nil {
					buf := new(bytes.Buffer)

					err := mdTemplates["/mandos/404.html"].Execute(buf, PageVars{
						Url: c.BaseURL()+c.OriginalURL(), Node: &nodeInfo, Ctx: c, Now: time.Now().Unix(),
					})
					if err!=nil { fmt.Println(err); return c.Status(500).SendString(err.Error()) };

					return c.Send(buf.Bytes())

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
				buf := new(bytes.Buffer)

				err := mdTemplates[templateRelPath].Execute(buf, PageVars{
					Url: c.BaseURL()+c.OriginalURL(), Node: &nodeInfo, Ctx: c,
				})
				if err != nil {
					log.Printf("Template Error: %v", err)
					return c.Status(500).SendString(err.Error())
				}
				return c.Send(buf.Bytes())

			}else{return c.SendString("No template found")}

		// If the wanted file is not markdown
		default:
			// Sanitize the user given urlPath.
			absPath := SafeJoin(notesPath, urlPath)
			if absPath==""{return c.SendStatus(404)}
			// If it is a hidden file, do not show it.
			if strings.HasPrefix(filepath.Base(absPath), ".") {return c.SendStatus(404)} 

			if !noAttCheck {
				// Prefer the cached attachment existence value.
				_, exists := attachmentExistenceCache.Get(absPath)
				if !exists {
					// Check if at least one node has a link to the attachment.
					err := attExistStmt.QueryRow(urlPath).Scan(&urlPath)
					if err != nil {
						if err == sql.ErrNoRows { return c.SendStatus(404) }
						log.Println("Database error:", err); return c.SendStatus(500)
					}
					attachmentExistenceCache.Set(absPath, struct{}{}, time.Second*30) // Save to the cache.
				}
			}

			// If we reach here, the attachment is found.
			c.Response().Header.Add("Cache-Control", "max-age=604800")
			return c.SendFile(absPath)
		}
	})

	return attExistStmt
}
