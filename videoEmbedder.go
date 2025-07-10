package main

import (
	"path/filepath"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Option is a functional option type for this extension.
type Option func(*embedExtension)

type embedExtension struct{}

// New returns a new Embed extension.
func VideoEmbedder(opts ...Option) goldmark.Extender {
	e := &embedExtension{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *embedExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(
		parser.WithASTTransformers(
			util.Prioritized(defaultASTTransformer, 500),
		),
	)
	m.Renderer().AddOptions(
		renderer.WithNodeRenderers(
			util.Prioritized(NewHTMLRenderer(), 500),
		),
	)
}

// VideoEmbed struct represents a static video embed of the Markdown text.
type VideoEmbed struct {ast.Image}

// KindYouTube is a NodeKind of the YouTube node.
var KindVideoEmbed = ast.NewNodeKind("videoEmbed")

// Kind implements Node.Kind.
func (n *VideoEmbed) Kind() ast.NodeKind {
	return KindVideoEmbed
}

// NewYouTube returns a new YouTube node.
func NewVideoEmbed(img *ast.Image) *VideoEmbed {
	c := &VideoEmbed{}
	c.Destination = img.Destination
	c.Title = img.Title

	return c
}

type astTransformer struct{}

var defaultASTTransformer = &astTransformer{}

func (a *astTransformer) Transform(node *ast.Document, reader text.Reader, pc parser.Context) {
	replaceImages := func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		if n.Kind() != ast.KindImage {
			return ast.WalkContinue, nil
		}

		img := n.(*ast.Image)
		ext := filepath.Ext(string(img.Destination))

		if ext != ".webm" && ext!=".mp4" && ext!=".mkv" {
			return ast.WalkContinue, nil
		}

		video := NewVideoEmbed(img)
		n.Parent().ReplaceChild(n.Parent(), n, video)

		return ast.WalkContinue, nil
	}

	ast.Walk(node, replaceImages)
}

// HTMLRenderer struct is a renderer.NodeRenderer implementation for the extension.
type HTMLRenderer struct{}

// NewHTMLRenderer builds a new HTMLRenderer with given options and returns it.
func NewHTMLRenderer() renderer.NodeRenderer {
	r := &HTMLRenderer{}
	return r
}

// RegisterFuncs implements NodeRenderer.RegisterFuncs.
func (r *HTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindVideoEmbed, r.renderVideoEmbed)
}

func (r *HTMLRenderer) renderVideoEmbed(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		return ast.WalkContinue, nil
	}

	video := node.(*VideoEmbed)

	w.Write([]byte(`<video src="` + string(video.Destination) + `" alt="`+ string(video.Title) +`" controls></video>`))
	return ast.WalkContinue, nil
}
