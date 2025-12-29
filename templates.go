package main

import (
	"bytes"
	"fmt"
	"hash/fnv"
	"log"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/mdigger/goldmark-attributes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/zenarvus/goldmark-bettermedia"
	"github.com/zenarvus/goldmark-headingid"
	"github.com/zenarvus/goldmark-mathjax"
)
var templateFuncs = template.FuncMap{
	"Add":func(x,y int)int{return x+y},
	"Sub":func(x,y int)int{return x-y},

	"ToStr":ToStr,
	"ToInt": ToInt,
	"ToHtml": ToHtml,

	"Query": Query,

	"ReplaceStr": StringReplacer,
	"Contains": strings.Contains,

	"DoubleSplitMap": DoubleSplitMap,
	"Split": Split,

	"AnyArr":AnySlice,

	"FormatDateInt": FormatDateInt,

	"GetNodeContent": GetNodeContent,
	"GetContentMatch": GetContentMatch,

	"ReadFile": ReadFile,
	"WriteFile": WriteFile,
	"DeleteFile": DeleteFile,

	"UrlParse":func(urlStr string)*url.URL{parsed,_:=url.Parse(urlStr); return parsed},

	"GetEnv": getEnvValue,
	
	"Include":IncludePartial,
}

var partialTemplates = make(map[string]*template.Template)
var mdTemplates = make(map[string]*template.Template)
var soloTemplates = make(map[string]*template.Template)
//initialize the template file
func loadAllTemplates(tType string){
	switch tType{
	case "md":
		mdTemplates = make(map[string]*template.Template)
		templatesPath := getEnvValue("MD_TEMPLATES")
		files, err := os.ReadDir(templatesPath); if err != nil {log.Fatal(err)}
		for _, file := range files {
			if !file.IsDir() { 
				relPath := strings.TrimPrefix(path.Join(templatesPath, file.Name()), notesPath)
				t,err := readTemplateFile(relPath)
				if err!=nil {log.Println(err)} else {mdTemplates[relPath] = t}

			}else if file.IsDir() && file.Name() == "partials" {
				partials, err := os.ReadDir(filepath.Join(templatesPath,"partials")); if err != nil {log.Fatal(err)}
				for _,partial := range partials {
					relPath := strings.TrimPrefix(path.Join(templatesPath, "partials", partial.Name()), notesPath)
					t,err := readTemplateFile(relPath)
					if err!=nil{log.Println(err)} else {partialTemplates[relPath] = t}
				}
			}
		}
		fmt.Println(len(mdTemplates),"markdown templates are loaded.")
	case "solo":
		filesStr:=getEnvValue("SOLO_TEMPLATES"); if filesStr==""{return}
		for relPath := range strings.SplitSeq(filesStr,",") {
			relPath = filepath.Join("/",relPath);
			t,err:=readTemplateFile(relPath)
			if err!=nil{log.Println(err)} else {soloTemplates[relPath]=t}
		}
		fmt.Println(len(soloTemplates),"solo templates are loaded.")
	}
}
func readTemplateFile(relPath string) (*template.Template, error) {
	tmplContent, err := os.ReadFile(filepath.Join(notesPath,relPath)); if err != nil {log.Fatal(err)}
	templ, err := template.New(relPath).Funcs(templateFuncs).Parse(string(tmplContent)); if err != nil{log.Fatal(err)}
	return templ, err
}
func loadTemplate(relPath, tType string) {
	tmpl, err := readTemplateFile(relPath)
	if err != nil {log.Fatal("Template error:",err)}
	switch tType {
	case "md": mdTemplates[relPath]=tmpl
	case "solo": soloTemplates[relPath]=tmpl
	case "partial": partialTemplates[relPath]=tmpl
	}
}

var htmlConverter = goldmark.New(
	attributes.Enable,
	goldmark.WithExtensions(extension.GFM, extension.Footnote, mathjax.MathJax, bettermedia.BetterMedia),
	goldmark.WithParserOptions(parser.WithAttribute(), parser.WithAutoHeadingID()),
	goldmark.WithRendererOptions(goldmarkHtml.WithHardWraps(), goldmarkHtml.WithXHTML(), goldmarkHtml.WithUnsafe()),
)
func ToHtml(mdText string) string {
	var html bytes.Buffer
	if err := htmlConverter.Convert([]byte(mdText), &html, parser.WithContext(parser.NewContext(parser.WithIDs(headingid.NewIDs())))); err != nil {log.Fatal(err)}
	return html.String()
}
func AnySlice(args ...any) (slice []any) {
	for _,arg := range args {slice = append(slice, arg)}
	return slice
}
func IncludePartial(partialName string)string{
	var buf bytes.Buffer

	if partial := partialTemplates[path.Join(getEnvValue("MD_TEMPLATES"), "partials", partialName)]; partial !=nil {
		err := partial.Execute(&buf, map[string]any{})
		if err!=nil{log.Println(err); return ""}

	}else{log.Println("Partial does not exists:", partialName); return ""}

	return buf.String()
}

// text should be something like this: "tags=hello|||tags=test|||year=2025|||author=mandos"
// First separate the items using the itemSepr, then, split the items into key-value pairs using keyValSepr.
// Then, create a key -> []values map.
func DoubleSplitMap(str any, itemSepr, keyValSepr string) map[string][]string {
	if str==nil{return map[string][]string{}}
	text := fmt.Sprint(str);
	items := strings.Split(text, itemSepr)

	var returnData = make(map[string][]string)
	
	for _,item := range items {
		// keyVal[0]: key, keyVal[1]: value
		keyVal := strings.Split(item, keyValSepr)
		if len(keyVal) < 2 {continue} // Skip if its not a key,value pair.

		returnData[keyVal[0]] = append(returnData[keyVal[0]], keyVal[1])
	}
	return returnData
}

func Split(str any, sepr string) []string {
	if str == nil {return []string{}}
	text := fmt.Sprint(str)
	if text=="" {return  []string{}}
	return strings.Split(text, sepr)
}

func ToStr(input any) string {
	return fmt.Sprint(input)
}
func ToInt(input string) (int, error) {
	i, err := strconv.Atoi(input)
	if err!=nil{log.Println("ToInt fail:", err); return 0, err}
	return i, err
}

func FormatDateInt(integer any, layout string) string {
	if unixDate,ok := integer.(int64); !ok {
		return "NaN"
	}else{
		return time.Unix(unixDate, 0).Format(layout)
	}
	
}

func StringReplacer(str string, oldNew ...string) string {
	replacer := strings.NewReplacer(oldNew...)
	return replacer.Replace(str)
}

//////////////////////// FILE READ-WRITE and DELETE /////////////////////////////////

// RWMutes is used to allow mutliple readings at the same time, while preventing reads while writing.
const shardCount = 64
type StripedLock struct {
	shards [shardCount]sync.RWMutex
}
var fileLocks StripedLock

// getShard picks a lock based on the filename hash
func (sl *StripedLock) getShard(key string) *sync.RWMutex {
	h := fnv.New32a()
	h.Write([]byte(key))
	index := h.Sum32() % shardCount
	return &sl.shards[index]
}

func ReadFile(filePath string) string {
	lock := fileLocks.getShard(filePath)
	lock.RLock()
	defer lock.RUnlock()

    contentBytes, err := os.ReadFile(filepath.Join(notesPath, filePath))
    if err != nil {
        log.Println("ReadFile error:", filePath, err)
        return ""
    }
    return string(contentBytes)
}

func WriteFile(filePath, content string) bool {
	lock := fileLocks.getShard(filePath)
	lock.Lock()
	defer lock.Unlock()

	folderPath := filepath.Dir(filePath)
	if folderPath != "." {
		err := os.MkdirAll(filepath.Join(notesPath, folderPath), 0755);
		if err!=nil {
			log.Fatalln("File directory could not be created.", err)
			return false
		}
	}

    err := os.WriteFile(filepath.Join(notesPath, filePath), []byte(content), 0644)
    if err != nil {
        log.Println("WriteFile error:", filePath, err)
        return false
    }
    return true
}

func DeleteFile(relPath string) bool {
	err := os.RemoveAll(filepath.Join(notesPath, relPath))
	if err != nil {log.Println("DeleteFile error:", relPath, err); return false}
	return true
}
