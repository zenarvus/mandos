# MaNDoS
Markdown Node Display Server

## Quick Start
### 1. Create a HTML Template
Create a folder named `mandos` in your Markdown folder.**

> `cd /path/to/markdown/folder && mkdir mandos`

Create a file named `main.html` within. This will be the default template used to serve the markdown files.

Example `main.html`:
```
<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>{{ToHtml .Content}}</body>
</html>
```

> Go to the `## Template Functions And Variables` section if you want to know more about what functions and variables you can use in a template file.

You can create as many templates as you want within this folder. To use them, add a `template` field to the metadata part of your markdown note and set the value as the name of the template.

Here is an example `template` metadata field with the value of `foo.html`:
```md
---
template: foo.html
---

# My Markdown Note
This node will be served in the foo.html template.
```

> The metadata part must be at the top of the markdown file, and must be formatted as YAML.

### 2. Create static Folder
You need to create a folder named `static` at the root of your Markdown folder.

`cd /path/to/markdown/folder && mkdir static`

- Files in this folder will **always** be served. This is where you should place your CSS and JavaScript files.
- Non-markdown files inside other directories are only served if they are linked in a public markdown file.

### 3 Run The Server
```bash
MD_FOLDER=/path/to/markdown/folder INDEX=index.md ONLY_PUBLIC=no MD_TEMPLATES=/path/to/templates/folder SOLO_TEMPLATES=rss.xml,node-list.json go -C /path/to/mandos run .
```

- `INDEX=index.md`: The file to be served at the root path of the server (`/`).
- `ONLY_PUBLIC=no`: Serve the every markdown file in the directory and consider all of them as `public`. Remove it if you just want to serve the `public` nodes.
    - A markdown file is considered public if its `public` metadata field is set to `true`.
    - Every non-markdown file a public markdown file links to will also be served.
    - However, private markdown files a public one links to will not be served.
- `MD_TEMPLATES=/path/to/templates/folder`: Set a custom template folder if you do not want to use the default `mandos` at the root of `MD_FOLDER`.
- `SOLO_TEMPLATES=rss.xml,node-list.json`: The paths of the files in `MD_FOLDER` to make them a "solo template."

> If you want to run the server with TLS encryption, you can use the `CERT` and `KEY` environment variables and pass the respective file paths to them.

## Template Functions And Variables
There are some variables and functions you can use inside a template. If it's a Markdown template, there are 9 basic variables you can use:

- `{{.Params}}`: The metadata part of the file, excluding the title, date and the tags. You can use `{{index .Params "key"}}` to access a value in it. `(map[string]any)`
- `{{.File}}`: The path of the markdown file, considering the `MD_FOLDER` as root. `(string)`
- `{{.Title}}`: The first H1 heading of the Markdown file or the `title` field in the document's metadata. `(string)`
- `{{.Date}}`: The date of the time aware node. You can make a node time-aware by adding a `date` field in its metadata with a value formatted as `yyyy-mm-dd`. `(time.Time)`
    - You can use `{{.Date.Format "format"}}` to convert it to any format you want.
    - Also you can check if the `.Date` field exists by using `{{if not .Date.IsZero}}...{{end}}`
- `{{.Tags}}`: The tags of a markdown file. To add a tag to a markdown file You can set a list of keywords in the `tags` metadata field. `([]string)`
- `{{.Content}}`: The raw content of the Markdown file, excluding the metadata part. `(string)`
- `{{.OutLins}}`: The .File values of the markdown files this one has links to. `([]string)`
- `{{.Inlinks}}`: The .File values of the markdown files with a link to this file. `([]string)`
- `{{.Attachments}}`: The Non-markdown files this file has links to. `([]string)`

The functions below can be used in both markdown templates and solo templates.

- `{{Add x y}}`: Add y to x. `(int)`
- `{{Sub x y}}`: Subtract y from x. `(int)`
- `{{ToHtml string}}`: Convert a raw markdown text to html. `(string)`
- `{{ReplaceStr str old new}}`: `strings.ReplaceAll` function. `(string)`
- `{{Contains str match}}`: `strings.Contains` function. `(bool)`
- `{{ListNodes}}`: returns a slice of nodes. A node contains the variables a markdown file has, expect the `.Content` field. You can iterate through them using `{{range ListNodes}}...{{end}}`. `([]*Node)`
- `{{SortNodesByDate ListNodes}}`: Returns a sorted slice of nodes by date. The newest will be the first. The date is derived from the `{{.Date}}` field of the node. `([]*Node)`

## Solo Templates
A solo template is a non-markdown file that can execute the template functions inside, when you navigate to its endpoint. These files can be used to, for example, create an RSS feed.

- You need to link the solo template in a markdown file to be able to serve it.
- The solo templates needs to be outside of the `static` folder, and they cannot be a markdown file.

Here is an example `node-list.json` file to create a node list in json format. It allows you to create [cool graphs](http://zenarvus.com/graph.md) like the ones in Obsidian.
```
{{- $listLen := len ListNodes -}}
[{{- range $i, $v := ListNodes -}}

{{- $olLen := len $v.OutLinks -}}
{{- $ilLen := len $v.InLinks -}}

{"file":"{{$v.File -}}",
"title":"{{ReplaceStr $v.Title `"` `\"` }}",
"tags":[
	{{- range $tagi,$tag := $v.Tags -}} "{{$tag}}"
		{{- if ne (Add $tagi 1) (len $v.Tags)}},{{end -}}
	{{- end -}}
],
"outlinks":[
	{{- range $oli, $olv := $v.OutLinks -}} "{{$olv}}"
		{{- if ne (Add $oli 1) $olLen }},{{end -}}
	{{- end -}}
],
"inlinks":[
	{{- range $ili, $ilv := $v.InLinks -}} "{{$ilv}}"
		{{- if ne (Add $ili 1) $ilLen }},{{end -}}
	{{- end -}}
]}{{if ne (Add $i 1) $listLen}},{{end -}}

{{- end -}}]
```

And here is an example `rss.xml` file to create an RSS feed.
```
<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel>
<title>Zenarvus</title>
<link>https://zenarvus.com/rss.xml</link>
<description>My second brain on the web.</description>
{{- range (SortNodesByDate ListNodes) -}}

{{- if not .Date.IsZero -}}
<item>
<title>{{.Title}}</title>
<link>https://zenarvus.com{{.File}}</link>
<pubDate>{{.Date.Format "Mon, 02 Jan 2006 15:04:05 GMT"}}</pubDate>
</item>
{{end}}

{{- end -}}
</channel></rss>
```

## Additional Tips
- After a file is created, updated or deleted, the change in the server side may take 5 seconds. Reloading nodes and attachments are somewhat an expensive operation and this time delay ensures only the last change will reload the nodes when bulk changes are made.
- You can use JavaScript in your Markdown files like in a HTML file.
- The server ignores the hidden markdown files (the ones with a dot at the start.)
- You can exclude a line in `ONLY_PUBLIC=yes` (default) mode by placing a `<!--exc-->` in the line.
