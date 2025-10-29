package main

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/mdigger/goldmark-attributes"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	goldmarkHtml "github.com/yuin/goldmark/renderer/html"
	"github.com/zenarvus/goldmark-bettermedia"
	"github.com/zenarvus/goldmark-mathjax"
)

var templateFuncs = template.FuncMap{
	"ToHtml": ToHtml,
	"ListNodes": ListNodes,
	"SortNodesByDate": SortNodesByDate,
	"FormatDate": FormatDate,
	"Add":func(x,y int)int{return x+y}, "Sub":func(x,y int)int{return x-y},
	"ReplaceStr": strings.ReplaceAll,
	"Contains": strings.Contains,
	"GetMetadata": GetMetadata,
}

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

func SortNodesByDate(nodes []*Node) []*Node {
    n := len(nodes)
    dup := append([]*Node(nil), nodes...)
    // keys: one string per element
    keys := make([]string, n)
    for i := range n {
        if s, ok := dup[i].Metadata["date"].(string); ok && s != "" { 
			keys[i] = s
        } else { keys[i] = "\xff" }
    }
    idx := make([]int, n)
    for i := range idx { idx[i] = i }

    sort.SliceStable(idx, func(a, b int) bool {
        return keys[idx[a]] > keys[idx[b]]
    })
    out := make([]*Node, n)
    for i, id := range idx { out[i] = dup[id] }
    return out
}

// Format yyyy-mm-dd format to any other format.
func FormatDate(timeStr string, targetFormat string) string {
	var parsedTime time.Time; var err error
	parsedTime, err = time.Parse("2006-01-02", timeStr)
	if err == nil {return parsedTime.Format(targetFormat)}
	return ""
}

// If the value exists, return it as string, if not, return an empty string
// example of the keys field: "servers.america.california.1"
func GetMetadata(metadata map[string]any, keyPath string) string {
	if keyPath == "" {return ""}
	keys := strings.Split(keyPath, ".")
	if len(keys) == 0 {return ""}

	var obj any = metadata[keys[0]]
	// If first key points into a map with the remaining key(s), allow starting from root when empty
	if len(keys) == 1 {if obj == nil {return ""}; return fmt.Sprint(obj)}

	for _, key := range keys[1:] {
		if obj == nil {return ""}

		switch obj.(type) {
		// If current object is a map[string]any, fetch next key
		case map[string]any: obj = obj.(map[string]any)[key]; continue
		// If current object is a slice/array, try to interpret key as an index
		case []string,[]int,[]bool,[]any:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 {return ""}
			// follow pointer indirections
			v := reflect.ValueOf(obj)
			for v.Kind() == reflect.Ptr { if v.IsNil() { return "" }; v = v.Elem() }
			if idx >= v.Len() {return ""}
			obj = v.Index(idx).Interface()
			continue
		// Convert other types to a string.
		default: return fmt.Sprint(obj)
		}
	}
	if obj == nil {return ""}
	return fmt.Sprint(obj)
}
