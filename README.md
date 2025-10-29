# MaNDoS
Markdown Node Display Server

The source code for my Markdown previewer and personal website.

## Quick Start
### 1.0 Create HTML Template
- **1.1: Create a folder named `mandos` in your Markdown folder.**
    > `cd /path/to/markdown/folder && mkdir mandos`
- **1.2: Create a file named `main.html` within. This will be the default template used to serve markdown files.**
    > You can include 6 variables in the markdown templates:
    > - `{{.Metadata}}` The metadata part of the file. {{index .Metadata "key name"}} to access a value in it.
    > - `{{.File}}`: Markdown's file name.
    > - `{{.Title}}`: The last H1 heading of the Markdown file or the `title` in the document's metadata.
    > - `{{.Content}}`: The raw content of the Markdown file, excluding metadata.
    > - `{{.OutLins}}`: It is a slice of strings. The strings are .File values of the markdown files this one has links to.
    > - `{{.Inlinks}}`: It is a slice of strings. The strings are .File values of the markdown files with a link to this file.

    > [!IMPORTANT]
    > To convert raw {{.Content}} to html, use {{ToHtml .Content}}

    > [!NOTE]
    > You can also use different templates for different Markdown files by specifying the template filename in the `template` metadata field.

### 2.0 Create static Folder
You need to create a folder named `static` at the root of your Markdown folder.

`cd /path/to/markdown/folder && mkdir static`

- Files in this folder will **always** be served. This is where you should place your CSS and JavaScript files.
- Non-markdown files inside other directories are only served if they are linked in a public markdown file.

### 3.0 Run The Server
`MD_FOLDER=/path/to/markdown/folder INDEX=index.md ONLY_PUBLIC=yes TEMPLATES=/path/to/templates/folder go -C /path/to/mandos run .`

- `INDEX=index.md`: This file will be served at the root path of the URL.
- `ONLY_PUBLIC=yes`: This option serves only public notes and their non-Markdown attachments. A note is considered public if its `public` metadata field is `true`.
- `TEMPLATES=/path/to/templates/folder`: Use this to set a custom template folder if you do not want to use the default `mandos` one at the root of your Markdown folder.

> If you want to run the server with TLS encryption, you can use the `CERT` and `KEY` environment variables by passing certificate and key file locations to them.

## Template Functions
- {{ListNodes}}: returns a slice of nodes. A node is a struct with these fields:
    - {{.File}}: Absolute path of the markdown file as a string, considering MD_FOLDER as root.
    - {{.Title}}: The last H1 heading or the "title" metadata field. It's a string
    - {{.Metadata}}: Metadata part of the markdown file. It is a map[string]string map. Use {{index .Metadata "key name"}} to access a value in it.
    - {{.OutLinks}}: It is a slice of strings. The strings are .File values of the markdown files this one has links to.
    - {{.InLinks}}: It is a slice of strings. The strings are .File values of the markdown files with a link to this file.
- {{SortNodesByDate ListNodes}}: Returns a sorted slice of nodes by date. Newest will be first.
- {{Add x y}}: Used to add y to x.
- {{Sub x y}}: Used to subtract y from x.
- {{ReplaceStr str old new}}: strings.ReplaceAll function.
- {{ToHtml string}}: Used to convert the raw markdown text to html.
- {{FormatTimeStr str}}: Used to format the `yyyy-mm-dd` time format to any other format.

## Additional Tips
- Metadata field must be at the top of your markdown file. Its syntax is similar to Hugo's. Place fields inside `+++` for `toml` and `---` for `yaml` metadata.
- You can use JavaScript in your Markdown files. It's supported.
- All non-Markdown file links in a public node are also made public.
- The server ignores the hidden files and folders (the ones with a dot at the start)
- You can place template codes inside `.xml` and `.json` files that are outside of the `static` folder.

Here is an example node-list.json file to create a node list in json format. It allows you to create [cool graphs](http://zenarvus.com/graph.md) like the ones in Obsidian.
```
{{ $listLen := len ListNodes }}
[{{range $i, $v := ListNodes}}{{$olLen := len $v.OutLinks}}{{$ilLen := len $v.InLinks}}
{"file":"{{$v.File}}",
"title":"{{ReplaceStr $v.Title `"` `\"` }}",
"outlinks":[{{range $oli, $olv := $v.OutLinks}}"{{$olv}}"{{if ne (Add $oli 1) $olLen }},{{end}}{{end}}],
"inlinks":[{{range $ili, $ilv := $v.InLinks}}"{{$ilv}}"{{if ne (Add $ili 1) $ilLen }},{{end}}{{end}}]}{{if ne (Add $i 1) $listLen}},{{end}}
{{end}}]
```

And here is an example rss.xml file to create an rss feed.
```
<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel>
<title>Zenarvus</title>
<link>https://zenarvus.com/rss.xml</link>
<description>My second brain on the web.</description>
{{range (SortNodesByDate ListNodes)}}{{$date := index .Metadata "date"}}{{if and $date (ne $date "")}}<item>
<title>{{.Title}}</title>
<link>https://zenarvus.com{{.File}}</link>
<pubDate>{{FormatTimeStr $date "Mon, 02 Jan 2006 15:04:05 GMT"}}</pubDate>
</item>{{end}}{{end}}
</channel></rss>
```

