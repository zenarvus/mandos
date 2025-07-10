# MaNDoS
Markdown Node Display Server

The source code for my Markdown previewer and personal website.

## Quick Start
### 1.0 Create HTML Template
- **1.1: Create a folder named `mandos` in your Markdown folder.**
    > `cd /path/to/markdown/folder && mkdir mandos`
- **1.2: Create a file named `main.html` within it. This will be the default template used.**
    > You can include 5 variables in the template:
    > - `{{.Host}}`: Hostname
    > - `{{.File}}`: Markdown's file name.
    > - `{{.Author}}`: The main author you specify when starting the server or the one defined in the `author` metadata.
    > - `{{.Title}}`: The last H1 heading of the Markdown file or the `title` in the document's metadata.
    > - `{{.Content}}`: The content of the Markdown file, excluding metadata.

    > [!INFO]
    > You can also use different templates for different Markdown files by specifying the template name in the `template` metadata field.

### 2.0 Create `static` Folder
You need to create a folder named `static` at the root of your Markdown folder.

`cd /path/to/markdown/folder && mkdir static`

- Files in this folder will always be served. This is where you should place your CSS and JavaScript files.

### 3.0 Run The Server
`go -C /path/to/mandos run . --md-folder=/path/to/markdown/folder --author=Zenarvus --index=index.md --only-public=yes --templates=/path/to/templates/folder`

- `--index=index.md`: This file will be served at the root path of the URL.
- `--only-public=yes`: This option serves only public notes and their non-Markdown attachments. A note is considered public if its `public` metadata field is `true`.
- `--templates=/path/to/templates/folder`: Use this to set a custom template folder if you do not want to use the default `mandos` one at the root of your Markdown folder.

## Special Endpoints
- `/rss`: This endpoint displays only time-aware nodes in RSS format. To make a Markdown node time-aware, include a `date` metadata field with a date value formatted as `yyyy-mm-dd`.
- `/node-list`: This endpoint shows which nodes are connected to each other. It allows you to create [cool graphs](http://zenarvus.com/graph.md) like the ones in Obsidian.
    > Example Format:
    > ```json
    > [{
    >     "file": "/1tvytN.md",
    >     "inlinks": [
    >         "/1tvytC.md"
    >     ],
    >     "outlinks": [
    >         "/1tvytC.md"
    >     ],
    >     "title": "Md-Agenda.Nvim Documentation"
    > },
    > {
    >     "file": "/1u0EI6.md",
    >     "inlinks": [
    >         "/1tvytC.md"
    >     ],
    >     "outlinks": [
    >         "/1tvytC.md"
    >     ],
    >     "title": "Md-Agenda Specification"
    > }]
    > ```

## Additional Tips
- Metadata field must be at the top of your markdown file. Its syntax is similar to Hugo's. Place fields inside `+++` for `toml` and `---` for `yaml` metadata.
- You can use JavaScript in your Markdown files. It's supported.
