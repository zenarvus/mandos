package main
import (
	"bytes"; "log"; "sort"; "strings"; "text/template"; "github.com/mdigger/goldmark-attributes"
	"github.com/yuin/goldmark"; "github.com/yuin/goldmark/extension"; "github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/zenarvus/goldmark-bettermedia"; "github.com/zenarvus/goldmark-mathjax"; "github.com/zenarvus/goldmark-headingid"
)
var templateFuncs = template.FuncMap{
	"Add":func(x,y int)int{return x+y}, "Sub":func(x,y int)int{return x-y},
	"ToHtml": ToHtml, "ListNodes": ListNodes, "SortNodesByDate": SortNodesByDate,
	"ReplaceStr": strings.ReplaceAll, "Contains": strings.Contains,
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
	for _, n := range servedNodes {
		if n != nil {nodes = append(nodes, n)}
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
