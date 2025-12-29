# MaNDoS
MaNDoS is a CMS server built for creating powerful markdown based wikis, digital gardens, blogs, documentations and more. With a flexible templating system, it allows you to generate beautiful note views, RSS feeds, JSON APIs, and even interactive guestbooks directly from your Markdown folder.

## Table of Contents
- [Quick Start](#quick-start)
    - [Installation](#installation)
    - [Creating a HTML Template](#creating-a-html-template)
    - [Creating a Static Folder](#creating-a-static-folder)
    - [Running The Server](#running-the-server)   
- [Environment Variables](#environment-variables)
- [Template Functions and Variables](#template-functions-and-variables)
    - [Variables](#variables)
    - [Functions](#functions)
- [Solo Templates](#solo-templates)
- [Database Tables](#database-tables)
- [Comparison With Hugo](#comparison-with-hugo)

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

# Or you can just use go run command (Don't do that it yet)
go -C /optional/path/to/mandos/code run .
```

### Creating a HTML Template
Create a folder named `mandos` in your Markdown folder.

```
cd /path/to/markdown/folder && mkdir mandos
``` 

Create a template named `main.html` within. This will be the default template used to serve the markdown files if no other one is specified.

Example `main.html`:

```
<!DOCTYPE html>
<html>
<head><title>{{.Title}}</title></head>
<body>{{ToHtml .Content}}</body>
</html>
```

You can create as many templates as you want within this folder. To use them, add a `template` field to the metadata part of your markdown note and set the value as the name of the template.

- The template named `404.html` will be used for 404 pages. If it does not exists, the default template will be used for that purpose.
- You can create partials inside `partials` directory in the template folder. To use them within other templates, use the `Include` function like this: `{{Include "example-partial.html"}}`


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

> This is a quoteblock with "test" class, "qb" id and custom styling. Mandos supports custom attributes!
{.test #qb style="background:black;color:white;"}

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
You need to create a folder named `static` at the root of your Markdown folder. Files in this folder will **always** be served. This is where you should place your CSS and JavaScript files.

```bash
cd /path/to/markdown/folder && mkdir static
```

- Non-markdown files inside other directories are only served if they are linked in a public markdown file.
- Hidden files (filenames starting with dot) are strictly not served. They can be used to store secret data about the server, and can be processed using `WriteFile`, `ReadFile` and `DeleteFile` options.

### Running The Server
Mandos uses environment variables for configuration. You can pass them directly like this:

```bash
MD_FOLDER=/path/to/markdown/folder INDEX=index.md ONLY_PUBLIC=no MD_TEMPLATES=/path/to/templates/folder SOLO_TEMPLATES=rss.xml,node-list.json,api/comment-guestbook RATE_LIMIT=api/comment-guestbook:600:3 CONTENT_SEARCH=true mandos
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
env -S $(grep -v '^#' /etc/mandos/config.env) mandos
```

## Evironment Variables
<details><summary>12 Environment Variables</summary>

### MD_FOLDER
- **Usage:** `MD_FOLDER=/abs/path/to/markdown/folder`
- **Description:** The folder to be used to serve the markdown nodes. The markdown files inside `static` or `mandos` folders, or the markdown files starting with dot will not be served regardless of the `ONLY_PUBLIC` value.
- **Default:** Empty string. Will throw an error if not specified.

### INDEX
- **Usage:** `INDEX=index.md`
- **Description:** The file to be served at the root path of the server (`/`). The default is 
- **Default:** `index.md`

### ONLY_PUBLIC
- **Usage:** `ONLY_PUBLIC=no`
- **Description:** Serve the every non-hidden markdown file in the directory and consider all of them as `public`. Remove it if you just want to serve the `public` nodes. A markdown file is considered public if its `public` metadata field is set to `true`. Every non-markdown file a public markdown file links to will also be served. However, private markdown files a public one links to will not be served.
- **Default:** `yes`

### NO_ATTACHMENT_CHECK
- **Usage:** `NO_ATTACHMENT_CHECK=true`
- **Description:** By default, Mandos serves non-markdown files only if they have an inlink from markdown files. Enabling this option disables this check and can improve attachment serving performance.
- **Default:** Empty string. Mandos checks if a non-markdown file contain an inlink before serving.

### MD_TEMPLATES
- **Usage:** `MD_TEMPLATES=/path/to/templates/folder`
- **Description:** Set a custom template folder if you do not want to use the default `mandos` at the root of `MD_FOLDER`.
- **Default:** `mandos` folder at the root of `MD_FOLDER`.

### SOLO_TEMPLATES
- **Usage:** `SOLO_TEMPLATES=rss.xml,node-list.json`
- **Description:** The relative paths of the files in `MD_FOLDER`, separated with commas. These files will be used as solo templates.
- **Default:** No solo template.

### CONTENT_SEARCH
- **Usage:** `CONTENT_SEARCH=true`
- **Description:** Enable searching through file contents using SQLite FTS5 virtual table.
- **Default:** `false`, No index will be generated, resulting in smaller database file sizes.

### CACHE_FOLDER
- **Usage:** `CACHE_FOLDER=/abs/path/to/cache/folder`
- **Description:** The location the SQLite database and other Mandos related files will be created.
- **Default:** `mandos` directory inside user's default cache folder.

### CERT and KEY
- **Usage:** `CERT=/abs/path/to/cert/file KEY=/abs/path/to/key/file`
- **Description:** Used to run the server with TLS encryption.
- **Default:** Ignored. The server will run in HTTP mode.

### BEHIND_PROXY
- **Usage:** `BEHIND_PROXY=true`
- **Description:** If it's set to true, the server will look at the `X-Forwarded-For` header for the IP addresses. Useful if you are behind a trusted gateway server (e.g. a load balancer). Do not forget to set this header inside your proxy server.
- **Default:** Not behind a proxy.

### RATE_LIMIT
- **Usage:** `RATE_LIMIT=!md:80:80,!att:80:80,solotemp1.json:80:45,solotemp2.txt:20:45`
- **Description:** Comma separated list of items for rate limiting endpoints. Each item is separated to three parts. The first one is can be `!md`, `!att` or the solo templates given in `SOLO_TEMPLATES`. `!md` is for all the markdown files and `!att` is for all attachments and static files. The second part is the expiration time of the limit in seconds, and the last part is the maximum number of recent connections during "expiration seconds" before sending a 429 response.
- **Default:** No rate limit is applied.
- **Warning:** Set `BEHIND_PROXY` if you are behind an another server.

### LOGGING
- **Usage:** `LOGGING=true`
- **Description:** Enable request logging and print IP addresses with access paths to STDOUT.
- **Default:** No logging.
</details>

## Template Functions And Variables
While the template functions can be used from any template, the scope of the variables differs.

### Variables
<details><summary>11 Core Variables</summary>

#### {{.Now}}
- **Scope:** Both in markdown and solo templates.
- **Description:** The current time in Unix epoch.
- **Type:** `int64`

#### {{.Ctx}}
- **Scope:** Both in markdown and solo templates.
- **Description:** See [Fiber CTX](https://docs.gofiber.io/api/ctx). It is not recommended to call `c.Send` or anything that writes a response to the requester. You should use it carefully.
- **Type:** `*fiber.Ctx`

#### {{.Url}}
- **Scope:** Both in markdown and solo templates.
- **Description:** The full URL path including the query. Example: `https://example.com/search?q=something`
- **Type:** `string`

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
</details>

### Functions
<details><summary>27 Core Functions</summary>

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
- **Usage:** `{{ToStr 100}} (Result: "100")`

#### {{ToInt string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Convert a string to integer. The first returned value is the converted integer. If the conversation fails, it returns `-9223372036854775808` which is equal to `{{.InvalidInt64}}`
- **Return:** `int64`
- **Usage:** `{{ToInt "100"}} (Result: 100)`

#### {{IsInt64Valid int64}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Check if an int64's value invalid. It returns `true` if the integer not equals to `-9223372036854775808`.
- **Return:** `bool`
- **Usage:** `{{IsInt64Valid 100}} (Result: true)`

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

#### {{HasPrefix string string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Check if the first parameter contains the second parameter as prefix.
- **Return:** `bool`
- **Usage:** `{{HasPrefix "Hello" "He"}} (Result: true)`

#### {{HasSuffix string string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Check if the first parameter contains the second parameter as suffix.
- **Return:** `bool`
- **Usage:** `{{HasSuffix "Hello" "lo"}} (Result: true)`

#### {{FilePathBase string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Get the filename from the given filepath.
- **Return:** `string`
- **Usage:** `{{FilePathBase "/this/is/a/long/path/.access-tokens"}} (Result: ".access-tokens")`

#### {{FilePathJoin string...}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Merge given parameters to one path string and sanitize them.
- **Return:** `string`
- **Usage:** `{{FilePathJoin "/sub" "folder", "test.txt"}} (Result: "/sub/folder/text.txt")`

#### {{Slugify string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Slugify the given string.
- **Return:** `string`
- **Usage:** `{{FilePathJoin "This Is A Title"}} (Result: "this-is-a-title")`

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
- **Description:** Get the raw markdown content of the given node, excluding the metadata and excluded lines.
- **Return:** `string`
- **Usage:** `{{GetNodeContent "/index.md"}}`

#### {{GetContentMatch string string int}}
- **Scope:** Both in markdown and solo templates.
- **Description:** First parameter must be the node content, and the second parameter must be the FTS5 match query used to match that node. The last parameter is the window size (characters). It highlights the matched part with the given window size in the line, and returns it.
- **Return:** `string`
- **Usage:** See the `search.json` example below.

#### {{Query string []any}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Make a SQLite query using the first parameter with values in second parameter. Return a slice of maps where keys are the selected columns and values are the values of these columns. It returns nothing if its not a SELECT query. You can iterate through them using `{{range (Query...)}}...{{end}}`.
- **Return:** `[]map[string]any`
- **Warning:** Always pass the values using the second parameter.
- **Usage:** See the examples below.

#### {{FileExists string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Check if a folder or file exists in `MD_FOLDER`. The given parameter must be the absolute file location, considering `MD_FOLDER` as root. It returns `true` if file exists.
- **Return:** `bool`
- **Usage:** `{{$exists := FileExists "/guestbook.txt"}} (Result: true)`

#### {{ReadFile string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Get the real content of any file in `MD_FOLDER`. The given parameter must be the absolute file location, considering `MD_FOLDER` as root. It returns nothing if file is empty or does not exists. If `WriteFile` runs at the same time, it waits for write before reading. 
- **Return:** `string`
- **Usage:** `{{$content := ReadFile "/guestbook.txt"}}`

#### {{WriteFile string string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Write to any file in `MD_FOLDER` and create if it does not exists. The first parameter must be the absolute file location, considering `MD_FOLDER` as root. It creates the sub folders if they do not exist. The second parameter must be the new file content. It returns `true` if the write is successful. Use with `ReadFile` instead of `GetNodeContent` if you are going to do a read & write operation. `GetNodeContent` does not wait for write to finish and it can result in corrupted files.
- **Return:** `bool`
- **Usage:** `{{WriteFile "/guestbook.txt" ("New line at the top\n" + $content)}}`

#### {{DeleteFile string}}
- **Scope:** Both in markdown and solo templates.
- **Description:** Delete any file or empty folder in `MD_FOLDER`. The given parameter must be the absolute file location, considering `MD_FOLDER` as root. It returns `true` if the delete is successful.
- **Return:** `bool`
- **Usage:** `{{DeleteFile "/old_guestbook.txt"}}`
</details>

## Solo Templates
A solo template is a non-markdown file that can execute the template functions and serve the results to GET and POST requests. These files can be used to, for example, generate an RSS feed, create a comment or delete a markdown file.

- The solo templates needs to be outside of the `static` folder, and they cannot be a markdown file.
- If the request method is POST, you can access to the form values in solo templates.

An example `nodes.json` file to create a node list in json format. It can allow you to create cool useless graphs like the ones in Obsidian.
```
{{- $queryStr := `WITH TargetNodes AS (
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

{{- $results := (Query $queryStr (AnyArr))  -}}
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

An example `rss.xml` file to create an RSS feed.
```
{{- $queryStr := `SELECT file, date, title FROM nodes WHERE date > 0 ORDER BY date DESC ;` -}}
<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom"><channel>
<title>Zenarvus</title>
<link>https://v.oid.sh/rss.xml</link>
<description>My second brain on the web.</description>
{{- range (Query $queryStr (AnyArr)) -}}

<item>
<title>{{.title}}</title>
<link>https://v.oid.sh{{.file}}</link>
<pubDate>{{FormatDateInt .date "Mon, 02 Jan 2006 15:04:05 GMT"}}</pubDate>
</item>

{{- end -}}
</channel></rss>
```

An example `search.json` to search file contents based on the value of the query parameter `q`. (CONTENT_SEARCH must be true)
```
{{- $queryStr := `SELECT n.file, n.date, n.title
FROM nodes n JOIN nodes_fts f ON n.id = f.rowid
WHERE nodes_fts MATCH ? ORDER BY bm25(nodes_fts, 10.0, 1.0) ASC LIMIT 20;` -}}
{{- $urlQ := (UrlParse .Url).Query.Get `q` -}}

{{- $results := (Query $queryStr (AnyArr $urlQ)) -}}
{{- $listLen := len $results -}}

[{{- range $i, $v := $results -}}

{"file":"{{$v.file -}}",
"title":"{{ReplaceStr $v.title `"` `\"` }}",
"content":"{{ReplaceStr (GetContentMatch (GetNodeContent $v.file) $urlQ 30) `\` `\\` `"` `\"` "\n" `\n` "\t" `\t`}}"
}{{if ne (Add $i 1) $listLen}},{{end -}}

{{- end -}}]
```

An example `api/comment-guestbook` file to append a text to the `guestbook.txt` file.
```
{{- $oldContent := ReadFile "/guestbook.txt" -}}
{{- $author := ReplaceStr (.Ctx.FormValue "author") "\n" `\n` -}}
{{- $comment := ReplaceStr (.Ctx.FormValue "comment") "\n" `\n` -}}
{{- if and (ne $comment "") (ne $author "") -}}
	{{- WriteFile "/guestbook.txt" (printf "%s: %s\n%s\n\n%s" (FormatDateInt .Now "02-Jan-2006") $author $comment $oldContent) -}}
{{- else -}}
{{ .Ctx.Status 400 }}
Comment or author name is not given.
{{- end -}}
```
- To prevent spam, set a rate limiter for this endpoint like: `RATE_LIMIT=api/comment-guestbook:600:3`. It only allows 3 requests within 600 seconds (5 minutes).

## Database Tables
The SQLite database contains five tables: `nodes`, `outlinks`, `attachments`, `params` and `nodes_fts`. It is possible to query nodes using these tables.

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

```
CREATE TABLE IF NOT EXISTS outlinks (
    "from" TEXT NOT NULL,
    "to"   TEXT NOT NULL,
    PRIMARY KEY ("from", "to"),
    FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_outlink_to ON outlinks("to");
```

```
CREATE TABLE IF NOT EXISTS attachments (
    "from" TEXT NOT NULL,
    file TEXT NOT NULL,
    PRIMARY KEY ("from", file)
    FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS idx_attachment_file ON attachments(file);
```

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
- **Hugo:** As a static site generator, content changes can require a full rebuild, and build times will increase as the website grows.
- **Mandos:** Operates as a live server. Content changes are automatically indexed and served in real time. It utilizes incremental builds, eliminating the need for a full rebuild step. This is more efficient than Hugo for large and/or dynamic websites.

**Built-In Dynamic Querying**
- **Hugo:** Only static content. External tools are required for dynamic queries and searches.
- **Mandos:** Built on SQLite with optional FTS5 (Full-Text Search) support. This allows for complex SQL queries and robust content searching directly within the templates.

**File-System Overhead**
- **Hugo:** Generates a full static copy of the original content, assets, and templates. Consequently, the required storage space is roughly double the size of the original source content.
- **Mandos:** Serves the original files directly and only copies metadata and content indexes for searching. The additional space required is usually significantly smaller than the original content.

**Serving Performance**
- **Hugo:** Files are pre-rendered. The server only needs to deliver static files from storage. When combined with caching, it becomes really fast.
- **Mandos:** Markdown is parsed on-the-fly, and the content needs a small amount of processing before being served every time. However, LRU and TTL caches are utilized and the difference in terms of performance is minimal with Hugo.

You should choose Hugo if you are unable to run a custom server, your content is mostly static, or you just feel like it. If the content constantly changes, it is better to choose Mandos.
