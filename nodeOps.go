package main

import (
	"bytes"; "fmt"; "runtime"; "io/fs"; "log"
	"os"; "path/filepath"; "strings"; "sync"; "time"
	"gopkg.in/yaml.v3"
)
// Key is the relative file location starting with slash, considering notesPath as root.
type Node struct {
	File *string // The same with the key. Only used in templates.
	Public bool // Is the file is public?
	Title string // The last H1 heading or the "title" metadata field.
	Date time.Time // the date metadata field.
	Tags []string // The tags field in the metadata. It must be an array.
	Content *string // Raw markdown content. Only used in templates.
	Params map[string]any // Fields in the YAML metadata part, except the title, public, date and tags.
	OutLinks []string // The list of nodes this node links to. (Their .File values)
	Attachments []string // Local non-markdown links in a node.
}
var servedNodes = make(map[string]*Node)
// The value is how many inlinks does that attachment have?
var servedAttachments = make(map[string]int)

var nmu sync.Mutex // Mutex for servedNodes
var amu sync.Mutex // Mutex for servedAttachments
func loadAllNodes(){
	servedNodes = make(map[string]*Node); servedAttachments = make(map[string]int)

	pathsCh := make(chan string, 256)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup

	startTime := time.Now()

	for i:=0; i <  runtime.NumCPU(); i++ {
		wg.Add(1)
		go func(){
			defer wg.Done()

			for npath := range pathsCh {
				nodeinfo, err := getNodeInfo(npath, true); if err != nil { select {case errCh <- err: default:} }

				if isServed(nodeinfo.Public) {
					relPath := strings.TrimPrefix(npath, notesPath);
					// Go maps are not safe for concurrent writes
					nmu.Lock(); servedNodes[relPath] = &nodeinfo; nmu.Unlock()
				}
			}

		}()
	}

	err := filepath.WalkDir(notesPath, func(npath string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		fileName := filepath.Base(d.Name())
		// Get only the non-hidden markdown files
		if !d.IsDir() && strings.HasSuffix(fileName, ".md") && !strings.HasPrefix(fileName,".") {
			pathsCh <- npath

		}else if d.IsDir() && d.Name() == "static" { return filepath.SkipDir }
		
		return nil
	})

	close(pathsCh)
	wg.Wait()
	
	if err != nil {fmt.Println("Error walking the path:", err)}

	// Using select case makes it unblocking.
	select {case e := <- errCh: fmt.Println("Error processing files:", e); default:}

	fmt.Println(len(servedNodes), "nodes are loaded (in", time.Since(startTime).Milliseconds(),"ms)")
	fmt.Println(len(servedAttachments),"attachments will be served.")
}

func loadNode(relPath string){
	oldNode,ok := servedNodes[relPath]
	if ok || oldNode != nil {
		// First decrease attachment link counts and delete them if the link count is 0.
		// As they can be changed when the content is changed. We will reload the new attachments in getFileInfo()
		for _,attachment := range oldNode.Attachments {
			servedAttachments[attachment]--
			if servedAttachments[attachment] == 0 {delete(servedAttachments, attachment)}
		}
	}

	absPath := filepath.Join(notesPath,relPath)
	nodeInfo,err := getNodeInfo(absPath, true); if err != nil {log.Println("Error while reloading the node: ",err.Error())}

	if !isServed(nodeInfo.Public) {removeNode(relPath); return}

	servedNodes[relPath]=&nodeInfo
}
func removeNode(relPath string) {
	node := servedNodes[relPath]
	// Delete the attachments if no node has a link to them.
	for _,attachment := range node.Attachments {
		servedAttachments[attachment]--
		if servedAttachments[attachment] == 0 {delete(servedAttachments, attachment)}
	}
	
	// Remove this node from other nodes' outlinks.
	for otherRelPath, node := range servedNodes {
		if otherRelPath != relPath {
			for i, outLink := range node.OutLinks {
				if outLink == relPath {
					// To delete the relPath from the outlinks of the nodes,
					// Write the last item to the relPath's location and remove the last item.
					node.OutLinks[i] = node.OutLinks[len(node.OutLinks)-1]
					node.OutLinks = node.OutLinks[:len(node.OutLinks)-1]
				}
			}
		}
	}

	delete(servedNodes, relPath)
}

func getNodeInfo(filename string, newNode bool) (nodeinfo Node, err error) {
	data, err := os.ReadFile(filename); if err != nil {return nodeinfo, err};

	var inMeta bool
	var inExcBlock bool
	var metaBuf bytes.Buffer
	var contentBuf bytes.Buffer
	var tagMap = make(map[string]struct{})
	var linkMap = make(map[string]struct{})
	nodeinfo.Title = filepath.Base(filename)

	for line := range bytes.SplitSeq(data, []byte("\n")) {
		// Exclude lines if ONLY_PUBLIC != "no"
		if onlyPublic != "no" {
			if !inExcBlock && bytes.Contains(line, []byte("<!--exc:start-->")){inExcBlock=true; continue}
			if inExcBlock && bytes.Contains(line, []byte("<!--exc:end-->")){inExcBlock=false; continue}
			if inExcBlock || bytes.Contains(line, []byte("<!--exc-->")){continue}
		}

		// Run the link finding regexp if the line contains "](" or "src="
		if bytes.Contains(line, []byte("](")) || bytes.Contains(line, []byte("src=")){
			for _,match := range linkRe.FindAllSubmatch(line,-1) {

				// Only look for internal links. (Not starting with [a-z]+://) Skip otherwise.
				if extLinkRe.Match(match[1]) {continue}

				// The link should start with a slash and the link path must consider notesPath as root. It should not be an absolute path or relative path inside the file system.
				linkMap[filepath.Join("/", string(match[1]))] = struct{}{}
			}
		}

		// Extract the metadata if the file starts with "---" (YAML metadata block)
		if contentBuf.Len()==0 && bytes.Equal(line, []byte("---")) && !inMeta { inMeta = true; continue }
		if inMeta {
			// If its end of the metadata, stop extracting.
			if bytes.Equal(line, []byte("---")) { inMeta = false; continue }
			// Otherwise, write the line to the buffer.
			metaBuf.Write(line)
			metaBuf.WriteByte('\n')
			continue
		}

		// Extract the title
		if bytes.HasPrefix(line, []byte("# ")){ nodeinfo.Title = strings.TrimPrefix(string(line), "# ") }

		// Write the lines to the content buffer
		contentBuf.Write(line)
        contentBuf.WriteByte('\n')
	}

	// Pass the content buffer to the fileinfo.Content if its not a new node. We do not use content field of the new nodes.
	if !newNode {nodeinfo.Content = new(string); *nodeinfo.Content = strings.TrimSuffix(contentBuf.String(), "\n")}

	// Add the tags in the tagMap to fileinfo.Tags
	for tag := range tagMap{nodeinfo.Tags=append(nodeinfo.Tags, tag)}

	// Add the links to the fileinfo.Attachments or fileinfo.Outlinks
	for link := range linkMap{
		// If its a markdown file, add to the outlinks.
		if strings.HasSuffix(link, ".md"){
			nodeinfo.OutLinks = append(nodeinfo.OutLinks, link)
		// If its a non-markdown file, add to the attachments
		} else {
			nodeinfo.Attachments = append(nodeinfo.Attachments, link)
			// If the node we get its info is a newly added node, increase the attachment's inlink count.
			// If the inlink count is 0, we need to stop serving the attachment and remove from the servedAttachments.
			if newNode {amu.Lock(); servedAttachments[link]++; amu.Unlock()}
		}
	}

	// Parse the YAML metadata to fileinfo.Params
    if metaBuf.Len() != 0 {
        if err := yaml.Unmarshal(metaBuf.Bytes(), &nodeinfo.Params); err != nil {
            return nodeinfo, fmt.Errorf("%s: %w", filename, err)
        }
    }

	// Get the public field in the metadata
	if isPublic,ok := nodeinfo.Params["public"].(bool); ok && isPublic {nodeinfo.Public = isPublic; delete(nodeinfo.Params, "public")}
	// Get the date of the node from the metadata
	if yamlDate,ok := nodeinfo.Params["date"].(time.Time); ok {nodeinfo.Date=yamlDate; delete(nodeinfo.Params,"date")}
	// Prefer the metadata title over the first header
	if mTitle,ok := nodeinfo.Params["title"].(string); ok && mTitle != "" { nodeinfo.Title = mTitle; delete(nodeinfo.Params,"title") }
	// Get the tags from the metadata
	if tags, ok := nodeinfo.Params["tags"].([]any); ok {
		for _,tag := range tags { nodeinfo.Tags = append(nodeinfo.Tags, fmt.Sprint(tag)) }
		delete(nodeinfo.Params, "tags")
	}

	return nodeinfo, nil
}
