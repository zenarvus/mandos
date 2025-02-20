package main

import (
	"fmt"
	"io"
	"log"
    "io/fs"
	"os"
    "regexp"
	"path"
    "bytes"
    "path/filepath"
	"strings"
    templater "text/template"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html/v2"

    "github.com/yuin/goldmark"
    "github.com/yuin/goldmark/extension"
    "github.com/yuin/goldmark/parser"
    "github.com/fsnotify/fsnotify"
    goldmarkHtml "github.com/yuin/goldmark/renderer/html"
)

var notesPath,_ = filepath.EvalSymlinks(getArgValue("--md-folder"))

var onlyPublic = getArgValue("--only-public")
var indexPage = getArgValue("--index")

var author = getArgValue("--author")

var servedFiles = make(map[string]bool)

var watcher *fsnotify.Watcher

func inServedCategory(content string) bool {
    if onlyPublic=="no" || strings.Contains(content, "<!--public-->") {return true}
	return false
}

var htmlConverter = goldmark.New(
    goldmark.WithExtensions(extension.GFM, extension.Footnote),
    goldmark.WithParserOptions(parser.WithAttribute()),
    goldmark.WithRendererOptions(
        goldmarkHtml.WithHardWraps(),
        goldmarkHtml.WithXHTML(),
        goldmarkHtml.WithUnsafe(),
    ),
)

var template *templater.Template
//initialize the template file
func init(){
    tmplContent, err := os.ReadFile(path.Join(notesPath, "/static/template.html"))
    if err!=nil{panic(err)}
    template, err = templater.New("body").Parse(string(tmplContent))
    if err!=nil{panic(err)}
}

//extract previewed links from markdown's content
func extractAttachments(content, baseDir string) {
	re := regexp.MustCompile(`!\[.*?\]\((.*)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
    matches = append(matches, regexp.MustCompile(`<.+src="/([^\"]+)".*?>`).FindAllStringSubmatch(content, -1)...)
	for _, match := range matches {
		attachmentPath := filepath.Join(baseDir, match[1])
		if _, err := os.Stat(attachmentPath); err == nil {
			servedFiles[strings.TrimPrefix(attachmentPath,toAbsolutePath(notesPath))] = true
		}
	}
}
func addWatchers(dir string) error {
	err := watcher.Add(dir)
	if err != nil {return err}
	// Add watchers for subdirectories
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		if d.IsDir() && path != dir {
			err = watcher.Add(path)
			if err != nil {return err}
		}
		return nil
	})
}

func loadNotesAndAttachments() {
	servedFiles = make(map[string]bool)

	err := filepath.WalkDir(notesPath, func(npath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			content, err := os.ReadFile(npath)
			if err != nil {
				return err
			}
			if inServedCategory(string(content)) {
				servedFiles[strings.TrimPrefix(npath,notesPath)]=true
				extractAttachments(string(content), toAbsolutePath(notesPath)/*filepath.Dir(path)*/)
			}
		}
		return nil
	})

	if err != nil {fmt.Println("Error walking the path:", err)}
}
// expandHomeDir replaces ~ with the user's home directory.
func expandHomeDir(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir(); if err != nil {return "", err}
		path = filepath.Join(homeDir, path[2:])
	}
	return path, nil
}

// toAbsolutePath converts a relative path to an absolute path.
func toAbsolutePath(path string) (string) {
	expandedPath, err := expandHomeDir(path); if err != nil {panic(err)}
	absolutePath, err := filepath.Abs(expandedPath); if err != nil {panic(err)}
	return absolutePath
}

func main() {
    //WATCHER START
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating file watcher:", err)
		return
	}
	defer watcher.Close()

	// Initial load of notes and attachments
	loadNotesAndAttachments()

	// Watch for changes in the directory and subdirectories
	err = addWatchers(notesPath)
	if err != nil {
		fmt.Println("Error adding watchers:", err)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Create == fsnotify.Create {
					// If a new directory is created, add it to the watcher
					fi, err := os.Stat(event.Name)
					if err == nil && fi.IsDir() {
						addWatchers(event.Name)
					}
				}
				// Reload notes and attachments when a file is added or modified
				loadNotesAndAttachments()
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				fmt.Println("Watcher error:", err)
			}
		}
	}()
    //WATCHER END

    engine := html.New(path.Join(notesPath, "/static"), ".html")
	app := fiber.New(fiber.Config{
        Views: engine,
    })

    //All files in static folder are served
    app.Static("/static", path.Join(notesPath,"/static"))

    //Only markdown files with <!--public--> metadata and their previewed attachments are served
    app.Get("/*", func(c *fiber.Ctx) error {
        urlPath := c.Path() //strings.TrimSuffix(c.Path(),"/")

        if !strings.Contains(urlPath,"/media/"){
            if urlPath=="/"{urlPath="/"+indexPage}

            returnedContent:="<p style='text-align:center;'>404 file does not exist</p>"
            returnedTitle:="404 Not Found"
            if servedFiles[urlPath]{
              returnedContent, returnedTitle = getFileContent(path.Join(notesPath,urlPath),c.Hostname())
            }

            // Render index template
            c.Response().Header.Add("Content-Type", "text/html")
            err := template.Execute(c.Response().BodyWriter(), map[string]interface{}{
                "Host": strings.Split(c.Hostname(),".")[0],
                "Content": returnedContent,
                "File": strings.TrimPrefix(urlPath,"/"),
                "Title": returnedTitle,
                "Author": author,
            })
            if err != nil {return c.Status(500).SendString("Error executing template")}
            return nil

        } else {
          if servedFiles[urlPath]{
            return c.SendFile(path.Join(notesPath,urlPath))
          }else{
            return c.SendStatus(404)
          }
        }
    })

    certFile:=getArgValue("--cert")
    keyFile:=getArgValue("--key")
    if certFile=="" || keyFile=="" {
        err := app.Listen(":"+getArgValue("--port"))
        if err!=nil{panic(err)}
    } else {
        err := app.ListenTLS(":"+getArgValue("--port"),certFile,keyFile)
        if err!=nil{panic(err)}
    }
}

func getArgValue(wantedArg string)string{
  args := os.Args
  returnValue := ""
  var available = map[string]bool{
    "--port":true,
    "--md-folder":true,
    "--cert":true,
    "--key":true,
    "--only-public":true,
    "--index":true,
    "--author":true,
  }
  for _,arg := range args {
    argKeyValue := strings.Split(arg,"=")
    if len(argKeyValue) == 2 && available[argKeyValue[0]]==true && argKeyValue[0]==wantedArg {
      return strings.TrimPrefix(strings.TrimSuffix(argKeyValue[1],"\""),"\"")
    
    }else if len(argKeyValue) == 1 && available[argKeyValue[0]]==true {
      log.Fatal(fmt.Errorf("Error in argument formatting"))
    }
  }

  //if user did not specified desired wantedArg
  if wantedArg=="--port"{returnValue="9700"
  }else if wantedArg=="--md-folder"{log.Fatal(fmt.Errorf("Please specify markdown folder path with --md-folder="))
  }else if wantedArg=="--index"{log.Fatal(fmt.Errorf("Please specify index page with --index="))
  }else if wantedArg=="--only-public"{returnValue="no"
  }else if wantedArg=="--author"{returnValue="author"}

  return returnValue
}

func getFileContent(filename, hostname string) (content string, title string) {
  if fileExists(filename) {
    // Open the file
    file, err := os.Open(filename)
    if err != nil {log.Fatal(err)}
    defer file.Close()

    // Read the file content into a byte slice
    content, err := io.ReadAll(file)
    if err != nil {log.Fatal(err)}

    contentStr := string(content)

    title := "Untitled"

    contentStrArr:=strings.Split(contentStr,"\n")
    var newContentLinesArr []string
    for _,line:=range contentStrArr {
        //get title
        if strings.HasPrefix(line, "# "){
            title = strings.TrimPrefix(line, "# ")
        }
        //remove excluded lines if the app run with --only-public=yes
        if onlyPublic != "no" {
            if strings.Contains(line,"<!--exc-->"){continue}
        }
        
        newContentLinesArr=append(newContentLinesArr,line)
      }
    contentStr=strings.Join(newContentLinesArr,"\n")

    //convert md to html
    var html bytes.Buffer
    if err := htmlConverter.Convert([]byte(contentStr), &html); err != nil {
        panic(err)
    }

    return html.String(), title
  
  } else {
    return "<p style='text-align:center;'>404 file does not exist</p>","404 Not Found"
  }

}

func fileExists(filePath string) bool {
    _, err := os.Stat(filePath)
    if os.IsNotExist(err) {
        return false
    }
    return err == nil
}
