package main

import (
	"os"
	"log"
	"regexp"
	"fmt"
	"io/fs"
	"io"
	"path"
	"path/filepath"
	"strings"
	templater "text/template"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

var notesPath = getNotesPath() //it does not and should not have a slash suffix.
var onlyPublic = getArgValue("--only-public")
var indexPage = getArgValue("--index")
var author = getArgValue("--author")

// filename -> {title/filename, date}
type servedFile struct {MapKey, Title, Date string}
var servedFiles = make(map[string]servedFile)

var templates = make(map[string]*templater.Template)
//initialize the template file
func initTemplates(){
	templatesPath := getArgValue("--templates")
	files, err := os.ReadDir(templatesPath)
	if err != nil {log.Fatal(err)}

	for _, file := range files {
		if !file.IsDir() { // Check if it's not a directory
			fmt.Println(file.Name())
			tmplContent, err := os.ReadFile(path.Join(templatesPath, file.Name()))
			if err != nil {log.Fatal(err)}
			template, err := templater.New(file.Name()).Parse(string(tmplContent))
			if err!=nil{log.Fatal(err)}

			templates[file.Name()] = template
		}
	}
}

func inServedCategory(metadataPublicField string) bool {
    if onlyPublic=="no" || metadataPublicField=="true" {return true}
	return false
}

//extract previewed links from markdown's content
func extractAttachments(content, baseDir string) {
	re := regexp.MustCompile(`!\[.*?\]\((.*)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
    matches = append(matches, regexp.MustCompile(`<.+src="/([^\"]+)".*?>`).FindAllStringSubmatch(content, -1)...)
	for _, match := range matches {
		attachmentPath := filepath.Join(baseDir, match[1])
		if _, err := os.Stat(attachmentPath); err == nil {
			servedFiles[strings.TrimPrefix(attachmentPath, notesPath)] = servedFile{
				MapKey: strings.TrimPrefix(attachmentPath, notesPath),
				Title: filepath.Base(attachmentPath),
			}
		}
	}
}

func loadNotesAndAttachments() {
	servedFiles = make(map[string]servedFile)

	err := filepath.WalkDir(notesPath, func(npath string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			fileinfo, err := getFileInfo(npath)
			if err != nil {return err}
			if inServedCategory(fileinfo.Metadata["public"]) {
				servedFiles[strings.TrimPrefix(npath, notesPath)] = servedFile{
					MapKey: strings.TrimPrefix(npath, notesPath),
					Title: fileinfo.Title,
					Date: fileinfo.Metadata["date"],
				}
				extractAttachments(fileinfo.Content, notesPath)
			}
		}
		return nil
	})

	if err != nil {fmt.Println("Error walking the path:", err)}
}

type fileInfo struct {
	Title string
	Content string
	Metadata map[string]string
}
func getFileInfo(filename string) (fileinfo fileInfo, err error) {
	if fileExists(filename) {
		// Open the file
		file, err := os.Open(filename)
		if err != nil {return fileinfo, err}
		defer file.Close()

		// Read the file content into a byte slice
		content, err := io.ReadAll(file)
		if err != nil {return fileinfo, err}

		contentStr := string(content)

		fileinfo.Title = "Untitled"

		contentStrArr:=strings.Split(contentStr,"\n")

		var inMetadataBlock bool
		var metadataType string //toml or yaml
		var metadataString string

		var newContentLinesArr []string
		for i,line:=range contentStrArr {
			//extract metadata
			if i==0 && (line == "---" || line == "+++"){
				inMetadataBlock = true
				switch line {
				case "---": metadataType = "yaml"
				case "+++": metadataType = "toml"
				}
				continue
			}
			if inMetadataBlock && i>0 && (line == "---" || line == "+++"){
				inMetadataBlock = false
				continue
			}
			if inMetadataBlock {metadataString += line+"\n"; continue}

			//extract title
			if strings.HasPrefix(line, "# "){
				fileinfo.Title = strings.TrimPrefix(line, "# ")
			}

			//remove excluded lines if the app run with --only-public=yes
			if onlyPublic != "no" {
				if strings.Contains(line,"<!--exc-->"){continue}
			}
        
			newContentLinesArr=append(newContentLinesArr,line)
		}
		fileinfo.Content=strings.Join(newContentLinesArr,"\n")

		//Parse the metadata string
		if metadataString != "" {
			switch metadataType {
			case "toml":
				err = toml.Unmarshal([]byte(metadataString), &fileinfo.Metadata)
				if err != nil {return fileinfo, err}
			case "yaml":
				err = yaml.Unmarshal([]byte(metadataString), &fileinfo.Metadata)
				if err != nil {return fileinfo, err}
			}
		}

		//Prefer the metadata title over the first header
		if fileinfo.Metadata["title"] != "" {
			fileinfo.Title = fileinfo.Metadata["title"]
		}

		return fileinfo, nil
  
	} else {
		fileinfo.Content = "<p style='text-align:center;'>404 file does not exist</p>"
		fileinfo.Title = "404 Not Found"
		return fileinfo, fmt.Errorf("404 Not Found")
	}

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

func fileExists(filePath string) bool {
    _, err := os.Stat(filePath)
    if os.IsNotExist(err) {
        return false
    }
    return err == nil
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
	"--templates": true, //The location of the templates. Default is mandos folder in the md-folder folder.
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
  }else if wantedArg=="--author"{returnValue="author"
  }else if wantedArg=="--templates"{returnValue=path.Join(getNotesPath(), "mandos")}

  return returnValue
}

func getNotesPath() string {
	notesPath, err := filepath.EvalSymlinks(getArgValue("--md-folder"))
	notesPath = toAbsolutePath(notesPath)
	if err!=nil{log.Fatal(err)}

	if strings.HasSuffix(notesPath, "/") {strings.TrimSuffix(notesPath, "/")}

	return notesPath
}
