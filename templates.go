package main
import (
	"bytes"; "log"; "fmt"; "sort"; "strings"; "text/template"; "os"; "path"; "path/filepath"; "github.com/mdigger/goldmark-attributes"
	"github.com/yuin/goldmark"; "github.com/yuin/goldmark/extension"; "github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/zenarvus/goldmark-bettermedia"; "github.com/zenarvus/goldmark-mathjax"; "github.com/zenarvus/goldmark-headingid"
)
var templateFuncs = template.FuncMap{
	"Add":func(x,y int)int{return x+y}, "Sub":func(x,y int)int{return x-y},
	"ToHtml": ToHtml, "ListNodes": ListNodes, "SortNodesByDate": SortNodesByDate,
	"ReplaceStr": strings.ReplaceAll, "Contains": strings.Contains,
}

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
func ListNodes() []*Node {
	nodes := make([]*Node, 0, len(servedNodes))
	for relPath, n := range servedNodes {
		if n != nil {n.File = &relPath; nodes = append(nodes, n)}
	}
	return nodes
}
func SortNodesByDate(nodes []*Node) []*Node {
	sort.SliceStable(nodes, func(i, j int) bool {
		a, b := nodes[i].Date, nodes[j].Date; ai, bj := a.IsZero(), b.IsZero()
		if ai != bj {return !ai && bj} /*non-zero before zero*/
		if a.Equal(b) {return false}
		return a.After(b)
	})
    return nodes
}
