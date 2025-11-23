package main
import (
	"bytes"; "log"; "fmt"; "strings"; "text/template"; "os"; "path"; "path/filepath"; "github.com/mdigger/goldmark-attributes";
	"github.com/yuin/goldmark"; "github.com/yuin/goldmark/extension"; "github.com/yuin/goldmark/parser"; "net/url"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/zenarvus/goldmark-bettermedia"; "github.com/zenarvus/goldmark-mathjax"; "github.com/zenarvus/goldmark-headingid"
)
var templateFuncs = template.FuncMap{
	"Add":func(x,y int)int{return x+y}, "Sub":func(x,y int)int{return x-y},
	"ToHtml": ToHtml, "GetNodes": GetNodes,
	"ReplaceStr": strings.ReplaceAll, "Contains": strings.Contains, "Split": strings.Split,
	"AnyArr":AnySlice, "BoolArr":BoolSlice, "Include":IncludePartial,
	"UrlParse":func(urlStr string)*url.URL{parsed,_:=url.Parse(urlStr); return parsed},
	"UrlParseQuery":func(urlStr string)url.Values{parsed,_:=url.ParseQuery(urlStr); return parsed},
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
	if err != nil {log.Fatal(err)}
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
func BoolSlice(args ...bool) (slice []bool) {
	for _,arg := range args {slice = append(slice, arg)}
	return slice
}
func IncludePartial(partialName string)string{
	var buf bytes.Buffer

	if partial := partialTemplates["/mandos/partials/"+partialName]; partial !=nil {
		err := partial.Execute(&buf, map[string]any{})
		if err!=nil{log.Println(err); return ""}

	}else{log.Println("Partial does not exists:", partialName); return ""}

	return buf.String()
}
