package main

import (
	"path/filepath"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// Option is a functional option type for this extension.
type Option func(*extender)

type extender struct{}

// New returns a new better media extension.
func BetterMediaExt(opts ...Option) goldmark.Extender {
	e := &extender{}
	for _, opt := range opts {opt(e)}
	return e
}

func (e *extender) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(NewHTMLRenderer(), 500),
		),
	)
}

// HTMLRenderer struct is a renderer.NodeRenderer implementation for the extension.
type HTMLRenderer struct{html.Config}

// NewHTMLRenderer builds a new HTMLRenderer with given options and returns it.
func NewHTMLRenderer() renderer.NodeRenderer {
	r := &HTMLRenderer{}
	return r
}

// RegisterFuncs implements NodeRenderer.RegisterFuncs.
func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	// run betterMediaRender function for every image element
	reg.Register(ast.KindImage, r.betterMediaRender)
}

func (r *HTMLRenderer) betterMediaRender(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {return ast.WalkContinue, nil}

	media := node.(*ast.Image)
	alt := filepath.Base(string(media.Destination))
	ext := filepath.Ext(alt)

	if ext == ".webm" || ext==".mp4" || ext==".mkv" {
		w.Write([]byte(`<video src="` + string(media.Destination) + `" alt="`+ alt +`" controls preload="metadata"></video>`))
	}else{
		w.Write([]byte(`<img src="` + string(media.Destination) + `" alt="`+ alt +`" loading="lazy"></img>`))
	}

	return ast.WalkSkipChildren, nil
}
