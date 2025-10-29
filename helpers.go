package main

import (
	"bytes"; "errors"; "fmt"; "io"; "io/fs"
	"log"; "os"; "path"; "path/filepath"; "regexp"
	"sort"; "strings"; "text/template"
	templater "text/template"; "time"

	"github.com/mdigger/goldmark-attributes"
	"github.com/pelletier/go-toml/v2"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/zenarvus/goldmark-bettermedia"
	"github.com/zenarvus/goldmark-mathjax"
	"gopkg.in/yaml.v3"
)

var notesPath = getNotesPath() //it does not and should not have a slash suffix.
var onlyPublic = getEnvValue("ONLY_PUBLIC")
var indexPage = getEnvValue("INDEX")

type Node struct {
	File string // Absolute file location, considering notesPath as root. It is the same value with the key of the node in the servedNodes map.
	Title string // The last H1 heading or the "title" metadata field.
	Content string // Raw markdown content
	Metadata map[string]string // Metadata part
	OutLinks []string // The list of nodes this node links to. (Their .File values)
	InLinks []string //The list of nodes contains a link to this node. (Their .File values)
}
var nodeList []*Node
var servedNodes = make(map[string]*Node)
var servedAttachments = make(map[string]bool)
func addNode(item *Node) {nodeList = append(nodeList, item); servedNodes[item.File] = item}
func cleanNodes(){servedNodes = make(map[string]*Node); nodeList=[]*Node{}}

var htmlConverter = goldmark.New(
	attributes.Enable,
	goldmark.WithExtensions(extension.GFM, extension.Footnote, mathjax.MathJax, bettermedia.BetterMedia),
	goldmark.WithParserOptions(parser.WithAttribute()),
	goldmark.WithRendererOptions(goldmarkHtml.WithHardWraps(), goldmarkHtml.WithXHTML(), goldmarkHtml.WithUnsafe()),
)
func ToHtml(mdText string) string {
	var html bytes.Buffer
	if err := htmlConverter.Convert([]byte(mdText), &html); err != nil {log.Fatal(err)}
	return html.String()
}
func ListNodes() []*Node {return nodeList}

//Time format should be yyyy-mm-dd
func FormatTimeStr(timeStr string, targetFormat string) (string) {
	timeStr = strings.ReplaceAll(timeStr, "/", "-")
	formats := []string{"2006-01-02","02-01-2006"}
	var parsedTime time.Time; var err error
	for _, format := range formats {
		parsedTime, err = time.Parse(format, timeStr)
		/*Return if parsing succeeds*/
		if err == nil {return parsedTime.Format(targetFormat)}
	}
	return ""
}
func SortNodesByDate(nodes []*Node) []*Node {
    dup := append([]*Node(nil), nodes...) // copy
    sort.SliceStable(dup, func(i, j int) bool {
        si := FormatTimeStr(dup[i].Metadata["date"], "20060102")
        sj := FormatTimeStr(dup[j].Metadata["date"], "20060102")
        // empty => oldest (put at end)
        if si == "" && sj == "" { return i < j } // stable fallback
        if si == "" { return false } // i is older => after j
        if sj == "" { return true }  // i is newer => before j
        return si > sj // newest first
    })
    return dup
}

var mdTemplates = make(map[string]*templater.Template)
var templateFuncs = template.FuncMap{
	"ToHtml": ToHtml,
	"ListNodes": ListNodes,
	"SortNodesByDate": SortNodesByDate,
	"FormatTimeStr": FormatTimeStr,
	"Add":func(x,y int)int{return x+y}, "Sub":func(x,y int)int{return x-y},
	"ReplaceStr": strings.ReplaceAll,
}
//initialize the template file
func loadMdTemplates(){
	mdTemplates = make(map[string]*templater.Template)
	templatesPath := getEnvValue("TEMPLATES")
	files, err := os.ReadDir(templatesPath)
	if err != nil {log.Fatal(err)}

	for _, file := range files {
		if !file.IsDir() { // Check if it's not a directory
			template,err := readTemplateFile(path.Join(templatesPath, file.Name()), file.Name())
			if err!=nil {log.Print(err)} else {mdTemplates[file.Name()] = template}
		}
	}
}
func readTemplateFile(path, name string) (*templater.Template, error) {
	tmplContent, err := os.ReadFile(path)
	if err != nil {log.Fatal(err)}
	template, err := templater.New(name).Funcs(templateFuncs).Parse(string(tmplContent))
	return template, err
}

func inServedCategory(metadataPublicField string) bool {
	if onlyPublic=="no" || metadataPublicField=="true" {return true}; return false
}

//extract non-markdown links from markdown's content
func extractAttachments(content, baseDir string) {
	re := regexp.MustCompile(`\[.*?\]\((.*)\)`)
	matches := re.FindAllStringSubmatch(content, -1)
	matches = append(matches, regexp.MustCompile(`<.+src="/([^\"]+)".*?>`).FindAllStringSubmatch(content, -1)...)
	for _, match := range matches {
		pathExt := path.Ext(match[1]); fileName := filepath.Base(match[1])
		//Skip the markdown files (thus, extract only the media) and hidden files.
		if pathExt == ".md" || strings.HasPrefix(fileName,"."){continue}

		absAttachmentPath := filepath.Join(baseDir, match[1])
		if _, err := os.Stat(absAttachmentPath); err == nil {
			relativeAPath := strings.TrimPrefix(absAttachmentPath, notesPath)
			servedAttachments[relativeAPath] = true
		}
	}
}

func loadNotesAndAttachments() {
	cleanNodes(); servedAttachments = make(map[string]bool)

	err := filepath.WalkDir(notesPath, func(npath string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		fileName := filepath.Base(d.Name())
		// Get only the non-hidden markdown files
		if !d.IsDir() && strings.HasSuffix(fileName, ".md") && !strings.HasPrefix(fileName,".") {
			fileinfo, err := getFileInfo(npath, true)
			if err != nil {return err}
			if inServedCategory(fileinfo.Metadata["public"]) {
				addNode(&Node{
					File: strings.TrimPrefix(npath, notesPath),
					Title: fileinfo.Title, Metadata: fileinfo.Metadata, OutLinks: fileinfo.OutLinks,
				})
				extractAttachments(fileinfo.Content, notesPath)
			}
		}
		return nil
	})
	if err != nil {fmt.Println("Error walking the path:", err)}
	SetInLinks()
}

func getFileInfo(filename string, includeConns bool) (fileinfo Node, err error) {
	if _, err := os.Stat(filename); err == nil {
		// Open the file
		file, err := os.Open(filename); if err != nil {return fileinfo, err}
		defer file.Close()
		// Read the file content into a byte slice
		content, err := io.ReadAll(file); if err != nil {return fileinfo, err}
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

		var newContentLinesArr []string; var gotTitle bool
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
				inMetadataBlock = false; continue
			}
			if inMetadataBlock {metadataString += line+"\n"; continue}

			//extract title
			if !gotTitle && strings.HasPrefix(line, "# "){
				fileinfo.Title = strings.TrimPrefix(line, "# "); gotTitle=true
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
	}

	return Node{},err
}

// Consider removing this and handle with inlinks in client side
func SetInLinks(){
	for _, fnode := range nodeList {
		for _, outLink := range fnode.OutLinks {
			// If outLink of the fnode exists in the served files
			if updatedStruct, ok := servedNodes[outLink]; ok && updatedStruct.File != "" {
				// update the outLink node's inlinks and add fnode's MapKey
				updatedStruct.InLinks = append(updatedStruct.InLinks, fnode.File)
				servedNodes[outLink]=updatedStruct
			}
		}
	}
}

func getEnvValue(key string)string{
	// If environment variable has a value, return it.
	if os.Getenv(key) != "" {return os.Getenv(key)}
	// If no value is assigned to the environment variable, use the default one or give an error.
	switch key {
	case "PORT": return "9700"
	case "ONLY_PUBLIC": return "no"
	//The location of the templates. Relative to the MD_FOLDER. Default is mandos.
	case "TEMPLATES": return path.Join(getNotesPath(), "mandos")
	case "MD_FOLDER": log.Fatal(fmt.Errorf("Please specify markdown folder path with MD_FOLDER environment variable."))
	case "INDEX": log.Fatal(fmt.Errorf("Please specify index file using INDEX environment variable"))
	}
	return ""
}

func getNotesPath() string {
	// Follow the system links and get the md-folder path.
	p, err := filepath.EvalSymlinks(getEnvValue("MD_FOLDER")); if err!=nil{log.Fatal(err)}
	// Replaces ~ with the user's home directory.
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir(); if err != nil {log.Fatal(err)}
		p = filepath.Join(home, p[2:])
	}
	// Converts a relative path to an absolute path.
	p, err = filepath.Abs(p); if err != nil {log.Fatal(err)}
	p = strings.TrimSuffix(p, "/")
	return p
}
