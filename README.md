# MaNDoS
Markdown Note Display Server

## Quick Start
### Installation
You can use the default release binaries, build it with `go build .` command, or just use `go run` command to start it.

```
git clone https://github.com/zenarvus/mandos
cd mandos

# If you want to build
go build . && chmod +x ./mandos
# Then you can execute the binary (Don't do that it yet)
./mandos

# Or just use go run command (Don't do that it yet)
go run .
```

### Creating a HTML Template
Create a folder named `mandos` in your Markdown folder.**

```
cd /path/to/markdown/folder && mkdir mandos
``` 

Create a template named `main.html` within. This will be the default template used to serve the markdown files.

Example `main.html`:

```
<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>{{ToHtml .Content}}</body>
</html>
```

> `404.html` template will be used for 404 pages. If it does not exists, the default template will be used for that purpose.

> You can create partials inside `mandos/partials` and use them with `{{Include partialName}}` inside the template.

> Go to the `## Template Functions And Variables` section if you want to know more about what functions and variables you can use in a template file.

You can create as many templates as you want within this folder. To use them, add a `template` field to the metadata part of your markdown note and set the value as the name of the template.

Here is an example markdown file with `template`, `tags`, `date` and `public` metadata fields:
```md
---
template: foo.html
tags: [blog, personal, test]
date: 2077-01-01
public: true
---

# My Markdown Note
Hello there. This node will be served in the foo.html template with given tags and date.

This line will be excluded if ONLY_PUBLIC=yes <!--exc-->

<!--exc:start-->
This block will be excluded if ONLY_PUBLIC=yes
- Useful if you want to make a page public, but some content is hidden.
<!--exc:end-->

File paths to other notes and links must be an absolute path, considering `MD_FOLDER` as root. For example, if your other note is in /home/user/md-folder/to/other-node.md, you should link it like this:
- [Other node](/to/other-node.md)

> The notes must not contain any space, "?" or "#" characters. Otherwise, things may break.

<script>
console.log("This script will be executed")
</script>
```

> The metadata part must be at the top of the markdown file, and must be formatted as YAML, inside `---` blocks.

### Creating a static Folder
You need to create a folder named `static` at the root of your Markdown folder.

```bash
cd /path/to/markdown/folder && mkdir static
```

- Files in this folder will **always** be served. This is where you should place your CSS and JavaScript files.
- Non-markdown files inside other directories are only served if they are linked in a public markdown file.

### 3 Run The Server
Mandos uses environment variables for configuration. You can pass them directly like this:

```bash
MD_FOLDER=/path/to/markdown/folder INDEX=index.md ONLY_PUBLIC=no MD_TEMPLATES=/path/to/templates/folder SOLO_TEMPLATES=rss.xml,node-list.json CONTENT_SEARCH=true go -C /path/to/mandos run .
```

Or you can create a configuration file:

```bash
# /etc/mandos/config.env

MD_FOLDER=/path/to/markdown/folder
INDEX=index.md
ONLY_PUBLIC=no
MD_TEMPLATES=/path/to/templates/folder
SOLO_TEMPLATES=rss.xml,node-list.json
CONTENT_SEARCH=true
```

Then pass them to Mandos like this:

``` bash
env -S $(grep -v '^#' /etc/mandos/config.env) go -C /path/to/mandos run .
```

#### Evironment Variables
##### MD_FOLDER
- **Usage:** `MD_FOLDER=/abs/path/to/markdown/folder`
- **Description:** The folder to be used to serve the markdown nodes. The markdown files inside `static` or `mandos` folders, or the markdown files starting with dot will not be served regardless of the `ONLY_PUBLIC` value.
- **Default:** Empty string. Will throw an error if not specified.

##### INDEX
- **Usage:** `INDEX=index.md`
- **Description:** The file to be served at the root path of the server (`/`). The default is 
- **Default:** `index.md`

##### ONLY_PUBLIC
- **Usage:** `ONLY_PUBLIC=no`
- **Description:** Serve the every non-hidden markdown file in the directory and consider all of them as `public`. Remove it if you just want to serve the `public` nodes. A markdown file is considered public if its `public` metadata field is set to `true`. Every non-markdown file a public markdown file links to will also be served. However, private markdown files a public one links to will not be served.
- **Default:** `yes`

##### MD_TEMPLATES
- **Usage:** `MD_TEMPLATES=/path/to/templates/folder`
- **Description:** Set a custom template folder if you do not want to use the default `mandos` at the root of `MD_FOLDER`.
- **Default:** `mandos` folder at the root of `MD_FOLDER`.

##### SOLO_TEMPLATES
- **Usage:** `SOLO_TEMPLATES=rss.xml,node-list.json`
- **Description:** The realtive paths of the files in `MD_FOLDER`, separated with commas, to make them solo templates.
- **Default:** Empty string.

##### CONTENT_SEARCH
- **Usage:** `CONTENT_SEARCH=true`
- **Description:** Enable searching through file contents using SQLite FTS5 virtual table.
- **Default:** `false`, No index will be generated, resulting in smaller database file sizes.

##### TRUSTED_PROXIES
- **Usage:** `TRUSTED_PROXIES=192.168.1.3,10.1.10.1`
- **Description:** A separated with comma list of trusted server ip addresses that are allowed to modify headers. This can be useful if your server is behind a load balancer or VPN, and you want to get the ip address of the real requester.
- **Default:** Empty string.

##### CACHE_FOLDER
- **Usage:** `CACHE_FOLDER=/abs/path/to/cache/folder`
- **Description:** The location the SQLite database and other Mandos related files will be created.
- **Default:** `mandos` directory inside user's default cache folder.

##### CERT and KEY
- **Usage:** `CERT=/abs/path/to/cert/file KEY=/abs/path/to/key/file`
- **Description:** Used to run the server with TLS encryption.
- **Default:** Ignored. The server will run in HTTP mode.

##### RATE LIMITING
- **Usage:** `MD_RATE_LIMIT_MAX=80 MD_RATE_LIMIT_EXPR=30 SOLO_RATE_LIMIT_MAX=25 SOLO_RATE_LIMIT_EXPR=45 ELSE_RATE_LIMIT_MAX=250 ELSE_RATE_LIMIT_EXPR=60`
- **Description:** Set the rate limiting for markdown files, solo templates and anything else. MAX is the maximum number of recent connections during EXPR seconds before sending a 429 response.
- **Default:** "0" for everything, meaning that no rate limiting is set.
- **Warning:** Set `TRUSTED_PROXIES` if you are behind another server.

## Template Functions And Variables
While the template functions can be used from any template, the scope of the variables differs.

### Variables
#### {{.Url}}
- **Scope:** Both in markdown and solo templates.
- **Description:** The full URL path including the query. Example: `https://example.com/search?q=something`
- **Type:** `string`

#### {{.Headers}}
- **Scope:** Both in markdown and solo templates.
- **Description:** The list of the headers of the request.
- **Type:** `map[string]string`

#### {{.Params}}
- **Scope:** Only in markdown templates.
- **Description:** The metadata part of the markdown file, excluding the title and date. The metadata must be a string or array of strings.
- **Type:** `map[string]any`

**Usage:** WIP
- You can use `{{index .Params "key"}}` to access a value in it.

#### {{.File}}
- **Scope:** Only in markdown templates.
- **Description:** The absolute path of the markdown file, considering the `MD_FOLDER` as root.
- **Type:** `string`

#### {{.Title}}
- **Scope:** Only in markdown templates.
- **Description:** The first H1 heading of the Markdown file, or the `title` field in the document's metadata.
- **Type:** `string`

#### {{.Date}}
- **Scope:** Only in markdown templates.
- **Description:** The Unix epoch date of the "time-aware" node. A node can be made time-aware by adding a `date` field in its metadata with a value formatted in `yyyy-mm-dd`.
- **Type:** `int64`

#### {{.Content}}
- **Scope:** Only in markdown templates.
- **Description:** The raw content of the Markdown file, excluding the metadata part.
- **Type:** `string`

#### {{.OutLinks}}
- **Scope:** Only in markdown templates.
- **Description:** The `.File` values of the markdown files this one has links to.
- **Type:** `[]string`

#### {{.Attachments}}
- **Scope:** Only in markdown templates.
- **Description:** The list of non-markdown files this file has links to.
- **Type:**  `[]string`

### Functions
#### {{Add int int}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Add second parameter to the first.
- **Return:** `int`
- **Usage:** `{{Add 1 3}} (Result: 3)`

#### {{Sub int int}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Subtract the second parameter from first.
- **Return:** `int`
- **Usage:** `{{Sub 5 3}} (Result: 2)`

#### {{ToStr any}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Convert `any` golang type to string using `fmt.Sprint`
- **Return:** `string`
- **Usage:** `{{Add 100}} (Result: "100")`

#### {{ToHtml any}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Convert the given Markdown string to HTML using Goldmark.
- **Return:** `string`
- **Usage:** `{{ToHtml "# Hello"}} (Result: "<h1>Hello</h1>")`

#### {{ReplaceStr string ...string }}
- **Scope:** Both in markdown and solo templates.
- **Description:** Replace the characters in the first parameter with the given "old" and "new" pairs.
- **Return:** `string`
- **Usage:** `{{ReplaceStr "Hello" "He" "Fe" "o" "a"}} (Result: "Fella")`

#### {{Contains string string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Check if the first parameter contains the second string
- **Return:** `bool`
- **Usage:** `{{Contains "Hello" "He"}} (Result: true)`

#### {{DoubleSplitMap any string string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Split the first parameter to individual items using the second parameter, then split these items to key-value pairs using the last parameter. If the keys are the same in different items combine their values into one `"key":["val1","val2"]` map item.
- **Return:** `map[string][]string`
- **Usage:** `{{DoubleSplitMap "key1=1||key1=2||key2=2||key3=3" "||" "="}} (Result: {"key1":["1",2], "key2":["2"], "key3":["3"]})`

#### {{Split any string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Split the first parameter using the second parameter.
- **Return:** `[]string`
- **Usage:** `{{DoubleSplitMap "key1=1||key1=2||key2=2||key3=3" "||"}} (Result: ["key1=1","key1=2","key2=2","key3=3"])`

#### {{AnyArr any...}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Convert given arguments in any type into a slice of any.
- **Return:** `[]any`
- **Usage:** `{{AnyArr "hello" 5}} (Result: ["hello", 5])`

#### {{UrlParse string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Golang `url.Parse` function without returning an error.
- **Return:** `*url.URL`
- **Usage:** `{{(UrlParse "https://example.com/search?q=test").Query.Get "q"}} (Result: "test")`

#### {{FormatDateInt any string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Convert an Unix epoch time to given date string format.
- **Return:** `string`
- **Usage:** `{{FormatDateInt 1766917944 "Mon, 02 Jan 2006 15:04:05 GMT"}}`

#### {{GetEnv string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Returns the value of the given environment variable, if it exists.
- **Return:** `string`
- **Usage:** `{{GetEnv "ADMIN_PASS"}} (Example-Result: "123456789")`

#### {{Include string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Include a partial to the template by passing the partial name as a string. (Do not use this function in the partials)
- **Return:** `string`
- **Usage:** `{{Include "partial.html"}}`

#### {{GetNodeContent string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Get the raw markdown content of the given node, excluding the metadata.
- **Return:** `string`
- **Usage:** `{{GetNodeContent "/index.md"}}`

#### {{GetContentMatch string string int}}
- **Scope:** Both in markdown and solo templates.
- **Description:** First parameter must be the node content, and the second parameter must be the FTS5 match query used to match that node. The last parameter is the window size (characters). It highlights the matched part with the given window size in the line, and returns it.
- **Return:** `string`
- **Usage:** See the `search.json` example below.

#### {{GetRows string []any}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Make a SQLite query using the first parameter with values in second parameter. Return a slice of maps where keys are the selected columns and values are the values of these columns. You can iterate through them using `{{range (GetRows...)}}...{{end}}`.
- **Return:** `[]map[string]any`
- **Warning:** Always pass the values using the second parameter.
- **Usage:** See the examples below.

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
<link>https://v.oid.sh/rss.xml</link>
<description>My second brain on the web.</description>
{{- range (GetRows $query (AnyArr)) -}}

<item>
<title>{{.title}}</title>
<link>https://v.oid.sh{{.file}}</link>
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
{{- $urlQ := (UrlParse .Url).Query.Get `q` -}}

{{- $results := (GetRows $query (AnyArr $urlQ)) -}}
{{- $listLen := len $results -}}

[{{- range $i, $v := $results -}}

{"file":"{{$v.file -}}",
"title":"{{ReplaceStr $v.title `"` `\"` }}",
"content":"{{ReplaceStr (GetContentMatch (GetNodeContent $v.file) $urlQ 30) `\` `\\` `"` `\"` "\n" `\n` "\t" `\t`}}"
}{{if ne (Add $i 1) $listLen}},{{end -}}

{{- end -}}]
```

## Tables
The sqlite contains 5 tables: "nodes", "outlinks", "attachments", "params" and "nodes_fts". It is possible to query nodes using these tables.

### Nodes
```
CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file TEXT UNIQUE,
		mtime INTEGER NOT NULL,
		date  INTEGER,
		title TEXT
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
- If a parameter has multiple values, the values are saved in different rows with the same key.

### Nodes_FTS
```
CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
    title, content,
    content='', contentless_delete=1,
    tokenize="unicode61 remove_diacritics 2 tokenchars '#'"
);
```
- Only created if `CONTENT_SEARCH` is true. Otherwise, does not exists.

## Comparison With Hugo
**No Full Rebuilds**
- **Hugo:** As a static site generator, content changes often require a full rebuild. Build times increase as the website grows.
- **Mandos:** Operates as a live server. Content changes are automatically indexed and served in real time. It utilizes incremental builds, eliminating the need for a full rebuild step. This is more efficient for large websites and frequent updates.

***Built-In Dynamic Querying**
- **Hugo:** Only static content. External tools are required for dynamic queries and searches.
- **Mandos:** Built on SQLite with optional FTS5 (Full-Text Search) support. This allows for complex SQL queries and robust content searching directly within the templates.

**File-System Overhead**
- **Hugo:** Generates a full static copy of the original content, assets, and templates. Consequently, the required storage space is roughly double the size of the original source content.
- **Mandos:** Serves the original files directly and only copies metadata and content indexes for searching. The additional space required is usually significantly smaller than the original content.

**Serving Performance**
- **Hugo:** Files are pre-rendered. The server only needs to deliver static files from storage. When combined with caching, it becomes really fast.
- **Mandos:** Markdown is parsed on-the-fly, and content needs a small amount of processing before being served every time. However, LRU and TTL caches are utilized and the difference in terms of performance is minimal compared to Hugo.

You should choose Hugo if you are unable to run a custom server, or just feel like it.
