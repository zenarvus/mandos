package main
import ("errors"; "fmt"; "io"; "io/fs"; "log"; "time"; "os"; "path"; "path/filepath"; "regexp"; "strings"; templater "text/template"; "gopkg.in/yaml.v3")
// Regular Expressions
var mdLinkRe = regexp.MustCompile(`\[.*?\]\(([^\)]*)\)`)
var htmlSrcRe = regexp.MustCompile(`<.+src="/([^\"]+)".*?>`)
var httpProtoRe = regexp.MustCompile(`^https?://`)

var notesPath = getNotesPath() //it does not and should not have a slash suffix.
var onlyPublic = getEnvValue("ONLY_PUBLIC")
var indexPage = getEnvValue("INDEX")
type Node struct {
	File string // Absolute file location, considering notesPath as root. It is the same value with the key of the node in the servedNodes map. It is not set in the struct returned by getFileInfo.
	Title string // The first H1 heading or the "title" metadata field.
	Date time.Time // the date metadata field.
	Tags []string // The value of the tags metadata field.
	Content string // Raw markdown content
	Params map[string]any // Fields in the YAML metadata part, except the title, date and tags
	OutLinks []string // The list of nodes this node links to. (Their .File values)
	InLinks []string //The list of nodes contains a link to this node. (Their .File values)
	Attachments []string // Local non-markdown links in a node.
}
var servedNodes = make(map[string]*Node)
// The value is how many inlinks does that attachment have?
var servedAttachments = make(map[string]int)

var mdTemplates = make(map[string]*templater.Template)
var soloTemplates = make(map[string]*templater.Template)
//initialize the template file
func loadTemplates(tType string){
	switch tType{
	case "md":
		mdTemplates = make(map[string]*templater.Template)
		templatesPath := getEnvValue("MD_TEMPLATES")
		files, err := os.ReadDir(templatesPath); if err != nil {log.Fatal(err)}
		for _, file := range files {
			if !file.IsDir() { 
				relPath := strings.TrimPrefix(path.Join(templatesPath, file.Name()), notesPath)
				t,err := readTemplateFile(relPath)
				if err!=nil {log.Println(err)} else {mdTemplates[relPath] = t}
			}
		}
		log.Println("Markdown templates are loaded:", len(mdTemplates))
	case "solo":
		filesStr:=getEnvValue("SOLO_TEMPLATES"); if filesStr==""{return}
		for _,relPath := range strings.Split(filesStr,",") {
			relPath = filepath.Join("/",relPath); t,err:=readTemplateFile(relPath)
			if err!=nil{log.Println(err)} else {soloTemplates[relPath]=t}
		}
		log.Println("Solo templates are loaded:", len(soloTemplates))
	}
}
func readTemplateFile(relPath string) (*templater.Template, error) {
	tmplContent, err := os.ReadFile(filepath.Join(notesPath,relPath)); if err != nil {log.Fatal(err)}
	template, err := templater.New(relPath).Funcs(templateFuncs).Parse(string(tmplContent)); if err != nil{log.Fatal(err)}
	return template, err
}
func loadTemplate(relPath, tType string) {
	tmpl, err := readTemplateFile(relPath)
	if err != nil {log.Fatal(err)}
	switch tType {
	case "md": mdTemplates[relPath]=tmpl
	case "solo": soloTemplates[relPath]=tmpl
	}
}

func inServedCategory(publicField bool)bool{return onlyPublic=="no" || publicField==true}

func loadNotesAndAttachments() {
	servedNodes = make(map[string]*Node); servedAttachments = make(map[string]int)
	err := filepath.WalkDir(notesPath, func(npath string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		fileName := filepath.Base(d.Name())
		// Get only the non-hidden markdown files
		if !d.IsDir() && strings.HasSuffix(fileName, ".md") && !strings.HasPrefix(fileName,".") {
			fileinfo, err := getFileInfo(npath, true); if err != nil {return err}
			mPublic, _ := fileinfo.Params["public"].(bool)
			if inServedCategory(mPublic) {
				fileinfo.File=strings.TrimPrefix(npath, notesPath); fileinfo.Content=""
				servedNodes[fileinfo.File] = &fileinfo
			}
		}
		return nil
	})
	if err != nil {fmt.Println("Error walking the path:", err)}
	SetInLinks()
	log.Println("Nodes are loaded:",len(servedNodes))
	log.Println("Attachments are loaded:",len(servedAttachments))
}
func loadNode(relPath string){
	absPath := filepath.Join(notesPath,relPath)
	nodeInfo,err := getFileInfo(absPath, true); if err != nil {log.Println("Error while reloading the node: ",err.Error())}
	nodeInfo.File=relPath; nodeInfo.Content=""

	mPublic, _ := nodeInfo.Params["public"].(bool)
	if !inServedCategory(mPublic) {return}

	// Set the inlinks of this node, made from other nodes.
	// This is very slow as we have to look for every node's outlinks in the map.
	// Consider creating a separate map for inlinks. The keys should be the each item in the outlinks of the each node.
	// Maybe I can use a number instead? It would be faster I guess. But then, I would have a hard time while trying to remove this node from other nodes' outlinks. Because I would not know which nodes have a link to this node without iterating them.
	var newInlinkArr []string
	for _, fnode := range servedNodes {
		if fnode.File != relPath {
			for _, outLink := range fnode.OutLinks {
				if outLink == relPath {
					newInlinkArr = append(newInlinkArr, fnode.File)
				}
			}
		}
	}
	nodeInfo.InLinks = newInlinkArr
	servedNodes[relPath]=&nodeInfo

	// Set the inlinks of other nodes, made from this node to them (because it can change when we update this node)
	for _, outlink := range nodeInfo.OutLinks {
		var alreadyContainsInlink bool
		otherNode,ok := servedNodes[outlink]
		if !ok || otherNode == nil {continue}

		for _,inlink := range otherNode.InLinks {
			if inlink==relPath {alreadyContainsInlink = true}
		}
		if !alreadyContainsInlink { otherNode.InLinks = append(otherNode.InLinks, nodeInfo.File) }
	}
}
func removeNode(relPath string) {
	node := servedNodes[relPath]
	// Delete the attachments if only one node (this one) has a link to them.
	for _,attachment := range node.Attachments {
		if servedAttachments[attachment] <= 1 {delete(servedAttachments, attachment)}
	}
	// Remove this node from the inlinks of other nodes.
	for _,outlink := range node.OutLinks {
		newInlinkArr := []string{}
		otherNode, ok := servedNodes[outlink]
        if !ok || otherNode == nil {continue}
		for _,inlink := range otherNode.InLinks {
			if inlink != node.File {newInlinkArr = append(newInlinkArr, inlink)}
		}
		otherNode.InLinks = newInlinkArr
	}

	// Remove this node from other nodes' outlinks.
	for _,inlink := range node.InLinks {
		newOutlinkArr := []string{}
		otherNode, ok := servedNodes[inlink]
		if !ok || otherNode == nil {continue}
		for _,outlink := range otherNode.OutLinks {
			if outlink != node.File {newOutlinkArr = append(newOutlinkArr, outlink)}
		}
		otherNode.OutLinks=newOutlinkArr
	}
	delete(servedNodes, relPath)
}

func getFileInfo(filename string, includeConns bool) (fileinfo Node, err error) {
	if _, err := os.Stat(filename); err == nil {
		// Open the file
		file, err := os.Open(filename); if err != nil {return fileinfo, err}; defer file.Close()
		// Read the file content into a byte slice
		content, err := io.ReadAll(file); if err != nil {return fileinfo, err}; contentStr := string(content)

		fileinfo.Title = filename
		var contentStrArr = strings.Split(contentStr,"\n")
		var metadataString string; var inMetadataBlock bool;

		var newContentLinesArr []string; var gotTitle bool
		for i,line:=range contentStrArr {
			//extract metadata
			if i==0 && (line == "---"){inMetadataBlock = true; continue}
			if inMetadataBlock && i>0 && (line == "---"){inMetadataBlock = false; continue}
			if inMetadataBlock {metadataString += line+"\n"; continue}
			//extract title
			if !gotTitle && strings.HasPrefix(line, "# "){
				fileinfo.Title = strings.TrimPrefix(line, "# "); gotTitle=true
			}
			//remove excluded lines if the app run with ONLY_PUBLIC=yes
			if onlyPublic != "no" && strings.Contains(line,"<!--exc-->") {continue}
		
			newContentLinesArr=append(newContentLinesArr,line)
		}
		fileinfo.Content=strings.Join(newContentLinesArr,"\n")
		// Extract Node Connections And Attachments
		if includeConns {
			// fileinfo.Content has no excluded lines. So it is safe to extract attachments from it.
			matches := mdLinkRe.FindAllStringSubmatch(metadataString+fileinfo.Content, -1)
			matches = append(matches, htmlSrcRe.FindAllStringSubmatch(contentStr, -1)...)
			tLinks := make(map[string]bool) // To check if the link is already inserted.
			for _, match := range matches {
				//If the file is not an http link
				if !httpProtoRe.MatchString(match[1]){
					// It should be the link path considering notesPath as root.
					linkLocation := filepath.Join("/", match[1])
					if !tLinks[linkLocation] {
						// If its a markdown file, add to the outlinks
						if strings.HasSuffix(linkLocation,".md"){fileinfo.OutLinks=append(fileinfo.OutLinks,linkLocation)
						// If its not a markdown file, add to the served attachments or increase its inlink count.
						}else{servedAttachments[linkLocation]++}
					}
					tLinks[linkLocation]=true
				}
			}
		}
		//Parse the metadata string
		if metadataString != "" {
			err = yaml.Unmarshal([]byte(metadataString), &fileinfo.Params)
			if err != nil {return fileinfo, errors.New(filename+": "+err.Error())}
		}
		// Get the date of the node
		if yamlDate,ok := fileinfo.Params["date"].(time.Time); ok {fileinfo.Date=yamlDate; delete(fileinfo.Params,"date")}
		// Get the tags from the metadata.
		fileinfo.Tags = ToStrArr(fileinfo.Params["tags"]); delete(fileinfo.Params,"tags")
		// Prefer the metadata title over the first header
		if mTitle,ok := fileinfo.Params["title"].(string); ok && mTitle != "" { fileinfo.Title = mTitle; delete(fileinfo.Params,"title") }

		return fileinfo, nil  
	}
	return Node{},err
}
func SetInLinks(){
	for _, fnode := range servedNodes {
		for _, outLink := range fnode.OutLinks {
			// If outLink of the fnode exists in the served files
			if updatedStruct, ok := servedNodes[outLink]; ok && updatedStruct.File != "" {
				// update the outLink node's inlinks and add fnode's MapKey
				updatedStruct.InLinks = append(updatedStruct.InLinks, fnode.File)
				servedNodes[outLink]=updatedStruct
			}
		}
	}
}
func getEnvValue(key string)string{
	// If environment variable has a value, return it.
	if os.Getenv(key) != "" {return os.Getenv(key)}
	// If no value is assigned to the environment variable, use the default one or give an error.
	switch key {
	case "MD_FOLDER": log.Fatal(fmt.Errorf("Please specify markdown folder path with MD_FOLDER environment variable."))
	case "INDEX": return "index.md"
	case "PORT": return "9700"
	case "ONLY_PUBLIC": return "no"
	//The location of the templates. Relative to the MD_FOLDER. Default is mandos.
	case "MD_TEMPLATES": return path.Join(getNotesPath(), "mandos")
	}
	return ""
}

func getNotesPath() string {
	// Follow the system links and get the md-folder path.
	p, err := filepath.EvalSymlinks(getEnvValue("MD_FOLDER")); if err!=nil{log.Fatal(err)}
	// Replaces ~ with the user's home directory.
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir(); if err != nil {log.Fatal(err)}
		p = filepath.Join(home, p[2:])
	}
	// Converts a relative path to an absolute path.
	p, err = filepath.Abs(p); if err != nil {log.Fatal(err)}
	return strings.TrimSuffix(p, "/")
}

func ToStrArr(obj any) []string {
	if obj == nil {return []string{}};
	switch v := obj.(type) {
	case []string: return v
	case []any: out := make([]string,0,len(v)); for _, x := range v {out = append(out, fmt.Sprint(x))}; return out
	case string,int,bool,float64,any: return []string{fmt.Sprint(v)}}
	return []string{}
}
