package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"bytes"
	"strings"
	"text/template"
	templater "text/template"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/zenarvus/goldmark-mathjax"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/mdigger/goldmark-attributes"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
)

var notesPath = getNotesPath() //it does not and should not have a slash suffix.
var onlyPublic = getEnvValue("ONLY_PUBLIC")
var indexPage = getEnvValue("INDEX")

// MapKey: Absolute file location, considering notesPath as root.
// Title: The last H1 heading or the "title" metadata field.
// Date: The "date" metadata field..
// Outlinks: The list of nodes this node links to. 
// Inlinks: The list of nodes contains a link to this node.
type servedFile struct {MapKey, Title string; Metadata map[string]string; InLinks, OutLinks []string}
var servedFiles = make(map[string]servedFile)

/*
type servedNode struct {Title string; Metadata map[string]string; InLinks, OutLinks []string}
var nodeList []*servedFile
var servedNodes = make(map[string]*servedFile)
var servedAttachments = make(map[string]string)
*/

var htmlConverter = goldmark.New(
	attributes.Enable,
	goldmark.WithExtensions(extension.GFM, extension.Footnote, mathjax.MathJax, BetterMediaExt()),
	goldmark.WithParserOptions(parser.WithAttribute()),
	goldmark.WithRendererOptions(goldmarkHtml.WithHardWraps(), goldmarkHtml.WithXHTML(), goldmarkHtml.WithUnsafe()),
)
func ConvertToHtml(mdText string) string {
	var html bytes.Buffer
	if err := htmlConverter.Convert([]byte(mdText), &html);
	err != nil {panic(err)}
	return html.String()
}

var templates = make(map[string]*templater.Template)
var templateFuncs = template.FuncMap{"ToHtml": ConvertToHtml}
//initialize the template file
func loadTemplates(){
	templates = make(map[string]*templater.Template)
	templatesPath := getEnvValue("TEMPLATES")
	files, err := os.ReadDir(templatesPath)
	if err != nil {log.Fatal(err)}

	for _, file := range files {
		if !file.IsDir() { // Check if it's not a directory
			tmplContent, err := os.ReadFile(path.Join(templatesPath, file.Name()))
			if err != nil {log.Fatal(err)}
			template, err := templater.New(file.Name()).Funcs(templateFuncs).Parse(string(tmplContent))
			if err!=nil{log.Fatal(err)}

			templates[file.Name()] = template
		}
	}
}

func inServedCategory(metadataPublicField string) bool {
	if onlyPublic=="no" || metadataPublicField=="true" {return true}
	return false
}

//extract non-markdown links from markdown's content
func extractAttachments(content, baseDir string) {
	re := regexp.MustCompile(`\[.*?\]\((.*)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
	matches = append(matches, regexp.MustCompile(`<.+src="/([^\"]+)".*?>`).FindAllStringSubmatch(content, -1)...)
	for _, match := range matches {
		pathExt := path.Ext(match[1])
		//Skip the markdown files (thus, extract only the media)
		if pathExt == ".md" || pathExt == ".mdx" {continue}

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
			fileinfo, err := getFileInfo(npath, true)
			if err != nil {return err}
			if inServedCategory(fileinfo.Metadata["public"]) {
				servedFiles[strings.TrimPrefix(npath, notesPath)] = servedFile{
					MapKey: strings.TrimPrefix(npath, notesPath),
					Title: fileinfo.Title,
					Metadata: fileinfo.Metadata,
					OutLinks: fileinfo.OutLinks,
				}
				extractAttachments(fileinfo.Content, notesPath)
			}
		}
		return nil
	})
	if err != nil {fmt.Println("Error walking the path:", err)}
	SetInLinks()
}

type fileInfo struct {
	Title string // The last H1 Heading or the "title" metadata field.
	Content string // Raw markdown content
	Metadata map[string]string // Metadata part
	OutLinks []string
}
func getFileInfo(filename string, includeConns bool) (fileinfo fileInfo, err error) {
	if fileExists(filename) {
		// Open the file
		file, err := os.Open(filename)
		if err != nil {return fileinfo, err}
		defer file.Close()

		// Read the file content into a byte slice
		content, err := io.ReadAll(file)
		if err != nil {return fileinfo, err}

		contentStr := string(content)

		//Node Connections
		if includeConns {
			linksRe := regexp.MustCompile(`\[.*?\]\((.*)\)`)
			matches := linksRe.FindAllStringSubmatch(contentStr, -1)
			matches = append(matches, regexp.MustCompile(`<.+src="/([^\"]+)".*?>`).FindAllStringSubmatch(contentStr, -1)...)
			for _, match := range matches {
				if !regexp.MustCompile(`^https?://`).MatchString(match[1]){
					fileinfo.OutLinks = append(fileinfo.OutLinks,
						strings.TrimPrefix(filepath.Join(notesPath, match[1]),notesPath))
				}
			}
		}

		fileinfo.Title = strings.TrimPrefix(filename,"/")

		contentStrArr:=strings.Split(contentStr,"\n")

		var metadataString string; var inMetadataBlock bool; var metadataType string //toml or yaml

		var newContentLinesArr []string
		var gotTitle bool
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
			if !gotTitle && strings.HasPrefix(line, "# "){
				fileinfo.Title = strings.TrimPrefix(line, "# ")
				gotTitle=true
			}

			//remove excluded lines if the app run with ONLY_PUBLIC=yes
			if onlyPublic != "no" && strings.Contains(line,"<!--exc-->") {continue}
		
			newContentLinesArr=append(newContentLinesArr,line)
		}
		fileinfo.Content=strings.Join(newContentLinesArr,"\n")

		//Parse the metadata string
		if metadataString != "" {
			switch metadataType {
			case "toml":
				err = toml.Unmarshal([]byte(metadataString), &fileinfo.Metadata)
				if err != nil {return fileinfo, errors.New(filename+": "+err.Error())}
			case "yaml":
				err = yaml.Unmarshal([]byte(metadataString), &fileinfo.Metadata)
				if err != nil {return fileinfo, errors.New(filename+": "+err.Error())}
			}
		}

		//Prefer the metadata title over the first header
		if fileinfo.Metadata["title"] != "" { fileinfo.Title = fileinfo.Metadata["title"] }

		return fileinfo, nil
  
	} else {
		fileinfo.Content = "<p>404 node does not exist.</p><p><a href=\"/\">Return To Index</a></p>"
		fileinfo.Title = "404 Not Found"
		return fileinfo, fmt.Errorf("404 Not Found")
	}

}

func SetInLinks(){
	for _, fnode := range servedFiles {
		for _, outLink := range fnode.OutLinks {
			updatedStruct := servedFiles[outLink]
			// If outLink of the fnode exists in the served files
			if updatedStruct.MapKey != "" {
				// update the outLink node's inlinks and add fnode's MapKey
				updatedStruct.InLinks = append(updatedStruct.InLinks, fnode.MapKey)
				servedFiles[outLink]=updatedStruct
			}
		}
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
	if os.IsNotExist(err) { return false }
	return err == nil
}

func getEnvValue(key string)string{
	// If environment variable has a value, return it.
	if os.Getenv(key) != "" {return os.Getenv(key)}
	// If no value is assigned to the environment variable, use the default one or give an error.
	switch key {
	case "PORT": return "9700"
	case "ONLY_PUBLIC": return "no"
	//The location of the templates. Default is mandos folder in the md-folder.
	case "TEMPLATES": return path.Join(getNotesPath(), "mandos")
	case "MD_FOLDER": log.Fatal(fmt.Errorf("Please specify markdown folder path with MD_FOLDER environment variable."))
	case "INDEX": log.Fatal(fmt.Errorf("Please specify index file using INDEX environment variable"))
	}
	return ""
}

func getNotesPath() string {
	notesPath, err := filepath.EvalSymlinks(getEnvValue("MD_FOLDER"))
	notesPath = toAbsolutePath(notesPath)
	if err!=nil{log.Fatal(err)}

	if strings.HasSuffix(notesPath, "/") {strings.TrimSuffix(notesPath, "/")}
	return notesPath
}
