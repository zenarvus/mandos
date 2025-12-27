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

> `404.html` template will be used for 404 pages.

> You can create partials inside `mandos/partials` and use them with `{{Include partialName}}` inside the template.

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
MD_FOLDER=/path/to/markdown/folder INDEX=index.md ONLY_PUBLIC=no MD_TEMPLATES=/path/to/templates/folder SOLO_TEMPLATES=rss.xml,node-list.json CONTENT_SEARCH=true go -C /path/to/mandos run .
```

- `INDEX=index.md`: The file to be served at the root path of the server (`/`). The default is `index.md`
- `ONLY_PUBLIC=no`: Serve the every markdown file in the directory and consider all of them as `public`. Remove it if you just want to serve the `public` nodes.
    - A markdown file is considered public if its `public` metadata field is set to `true`.
    - Every non-markdown file a public markdown file links to will also be served.
    - However, private markdown files a public one links to will not be served.
- `MD_TEMPLATES=/path/to/templates/folder`: Set a custom template folder if you do not want to use the default `mandos` at the root of `MD_FOLDER`.
- `SOLO_TEMPLATES=rss.xml,node-list.json`: The paths of the files in `MD_FOLDER` to make them a "solo template."
- `CONTENT_SEARCH=true`: Enable searching through file contents using SQLite FTS5.

> If you want to run the server with TLS encryption, you can use the `CERT` and `KEY` environment variables and pass the respective file paths to them.

## Tables
The sqlite contains 5 tables

"nodes", "outlinks", "attachments", "params" and "nodes_fts"

You can query nodes based on these.

### Nodes
```
CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file TEXT UNIQUE,
		mtime INTEGER NOT NULL,
		date  INTEGER,
		title TEXT,
);
CREATE INDEX IF NOT EXISTS idx_node_file ON nodes(file);
CREATE INDEX IF NOT EXISTS idx_node_date ON nodes(date);
```

### Outlinks
```
CREATE TABLE IF NOT EXISTS outlinks (
    "from" TEXT NOT NULL,
    "to"   TEXT NOT NULL,
    PRIMARY KEY ("from", "to"),
    FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_outlink_to ON outlinks("to");
```

### Attachments
```
CREATE TABLE IF NOT EXISTS attachments (
    "from" TEXT NOT NULL,
    file TEXT NOT NULL,
    PRIMARY KEY ("from", file)
    FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_attachment_file ON attachments(file);
```

### Params
```
CREATE TABLE IF NOT EXISTS params (
    "from"  TEXT NOT NULL,
    key   TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY ("from", key, value)
    FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_params_key_val_from ON params(key, value, "from");
CREATE INDEX IF NOT EXISTS idx_params_from ON params("from");
```

### Nodes_FTS
```
CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
    title, content,
    content='',
    tokenize="unicode61 remove_diacritics 2 tokenchars '#'"
);
```
- Only created if CONTENT_SEARCH is true. Otherwise, does not exists.

## Template Functions And Variables
There are some variables and functions you can use inside a template. If it's a Markdown template, there are 9 basic variables you can use:

- `{{.Params}}`: The metadata part of the file, excluding the title and date. You can use `{{index .Params "key"}}` to access a value in it. Must be a string or array of strings. `(map[string]any)`
- `{{.File}}`: The path of the markdown file, considering the `MD_FOLDER` as root. `(string)`
- `{{.Title}}`: The first H1 heading of the Markdown file, or the `title` field in the document's metadata. `(string)`
- `{{.Date}}`: The unix epoch date of the time aware node. You can make a node time-aware by adding a `date` field in its metadata with a value formatted as `yyyy-mm-dd`. `(int64)`
- `{{.Content}}`: The raw content of the Markdown file, excluding the metadata part. `(string)`
- `{{.OutLinks}}`: The .File values of the markdown files this one has links to. `([]string)`
- `{{.Attachments}}`: The Non-markdown files this file has links to. `([]string)`

***

- `{{.Url}}`: Return the full URL path including the query. Example: `https://example.com/search?q=something` This variable can be used in solo templates too. (string)

The functions below can be used in both markdown templates and solo templates.

- `{{Add x y}}`: Add y to x. `(int)`
- `{{Sub x y}}`: Subtract y from x. `(int)`
- `{{ToStr any}}`: Convert anything to string. `(string)`
- `{{ToHtml string}}`: Convert a raw markdown text to html. `(string)`
- `{{ReplaceStr str old new}}`: `strings.ReplaceAll` function. `(string)`
- `{{Contains str match}}`: `strings.Contains` function. `(bool)`
- `{{DoubleSplitMap anyStr itemSep keyValSep}}`: Split the string to individual items using itemSep, then split these items to key-value pairs using keyValSep. If the keys are the same in different items combine their values into one key->[val1,val2] map item. `(map[string][]string)`
- `{{Split anyStr seperator}}`: Split the given string using the separator. `(string)`
- `{{AnyArr (val1 val2)}}`: Convert given arguments of any types into a slice of any. `([]any)`
- `{{UrlParse urlStr}}`: `url.Parse` function but does not return error. `(*url.URL)`
- `{{UrlParseQuery urlStr}}`: `url.ParseQuery` function but does not return error. `(url.Values)`
- `{{FormatDateInt anyInt format}}`: Convert an unix epoch time to given date string format. `(string)`
- `{{Include partialNameStr }}`: Include a partial to the template by giving its name. Put the partial name in quotes (\`\`) (Do not use this function in the partials) `(string)`

***

- `{{GetRows queryStr AnyArr}}`: returns a slice of maps where keys are the selected columns and values are the values of these columns. You can iterate through them using `{{range (GetRows...)}}...{{end}}`. `([]map[string]any)`
    - `queryStr` must be a complete SQLite query using the aforementioned tables.
    - `AnyArr` must be the values being passed to the query.
    - See the examples below.

<!--
To get nodes that contain both "articles" and "moc" tag:

JOIN params AS p ON n.id = p."from"
WHERE p.key = 'tags' AND p.value IN ('public', 'article')
GROUP BY n.id, n.date, n.title HAVING COUNT(DISTINCT p.value) = 2
-->

## Solo Templates
A solo template is a non-markdown file that can execute the template functions inside, when you navigate to its endpoint. These files can be used to, for example, create an RSS feed.

- You need to link the solo template in a markdown file to be able to serve it.
- The solo templates needs to be outside of the `static` folder, and they cannot be a markdown file.

Here is an example `nodes.json` file to create a node list in json format. It allows you to create [cool graphs](http://v.oid.sh/graph.md) like the ones in Obsidian.
```
{{- $query := `WITH TargetNodes AS (
	SELECT n.file, n.date, n.title
	FROM nodes AS n
		
), TagsAgg AS (
	SELECT p."from", GROUP_CONCAT(p.key || '=' || p.value, '||') as params_str
	FROM params AS p
	INNER JOIN TargetNodes tn ON p."from" = tn.file AND p.key = "tags"
	GROUP BY p."from"
), OutlinksAgg AS (
    SELECT o."from", GROUP_CONCAT(o."to", '||') as outlinks_str
    FROM outlinks AS o
    INNER JOIN TargetNodes tn ON o."from" = tn.file
    GROUP BY o."from"
)
SELECT tn.file, tn.date, tn.title, par.params_str, ol.outlinks_str
FROM TargetNodes AS tn
LEFT JOIN TagsAgg AS par ON tn.file = par."from" LEFT JOIN OutlinksAgg AS ol ON tn.file = ol."from";` -}}

{{- $results := (GetRows $query (AnyArr))  -}}
{{- $listLen := len $results -}}

[
{{- range $i, $v := $results -}}

{{- $outlinks := (Split $v.outlinks_str "||") -}}
{{- $olLen := len $outlinks -}}

{{- $params := (DoubleSplitMap $v.tags_str "||" "=") -}}
{{- $tags := $params.tags -}}

{
	"file":"{{$v.file -}}",
	"title":"{{ReplaceStr $v.title `"` `\"` }}",
	"tags":[
		{{- range $tagi,$tag := $tags -}} "#{{$tag}}"
			{{- if ne (Add $tagi 1) (len $tags)}},{{end -}}
		{{- end -}}
	],
	"outlinks":[
		{{- range $oli, $olv := $outlinks -}} "{{$olv}}"
			{{- if ne (Add $oli 1) $olLen }},{{end -}}
		{{- end -}}
	]
}{{- if ne (Add $i 1) $listLen}},{{end -}}

{{- end -}}
]
```

And here is an example `rss.xml` file to create an RSS feed.
```
{{- $query := `SELECT file, date, title FROM nodes WHERE date > 0 ORDER BY date DESC ;` -}}
<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel>
<title>Zenarvus</title>
<link>https://zenarvus.com/rss.xml</link>
<description>My second brain on the web.</description>
{{- range (GetRows $query (AnyArr)) -}}

<item>
<title>{{.title}}</title>
<link>https://example.com{{.file}}</link>
<pubDate>{{FormatDateInt .date "Mon, 02 Jan 2006 15:04:05 GMT"}}</pubDate>
</item>

{{- end -}}
</channel></rss>
```

Finally, here is an example `search.json` to search file contents based on the value of the query parameter `q`. (CONTENT_SEARCH must be true)
```
{{- $query := `SELECT n.file, n.date, n.title
FROM nodes n JOIN nodes_fts f ON n.id = f.rowid
WHERE nodes_fts MATCH ? ORDER BY bm25(nodes_fts, 10.0, 1.0) ASC LIMIT 20;` -}}

{{- $results := (GetRows $query (AnyArr ((UrlParse .Url).Query.Get `q`))) -}}
{{- $listLen := len $results -}}

[{{- range $i, $v := $results -}}

{"file":"{{$v.file -}}",
"title":"{{ReplaceStr $v.title `"` `\"` }}",
"content":"{{ReplaceStr (ReplaceStr (ReplaceStr (ReplaceStr $v.content `\` `\\`) `"` `\"`) "\n" `\n`) "\t" `\t`}}"
}{{if ne (Add $i 1) $listLen}},{{end -}}

{{- end -}}]
```

## Additional Tips
- You can use JavaScript in your Markdown files like in a HTML file.
- The server ignores the hidden markdown files (the ones with a dot at the start.)
- You can exclude a line by placing a `<!--exc-->` in the line.
- Or you can exclude a block of lines by placing `<!--exc:start-->` and `<!--exc:end-->` between the lines.
- Links to other nodes or attachments must start with "/" and they all should be in the `MD_FOLDER`. File names should not contain any space, "?" or "#" character.
