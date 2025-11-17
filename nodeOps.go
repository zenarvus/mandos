package main

import (
	"bytes"; "fmt"; "runtime"; "io/fs"; "log"
	"os"; "path/filepath"; "strings"; "sync"; "time"
	"gopkg.in/yaml.v3"
)

type Node struct {
	File string // Absolute file location, considering notesPath as root. It is the same value with the key of the node in the servedNodes map. It is not set in the struct returned by getFileInfo.
	Public bool // Is the file is public?
	Title string // The last H1 heading or the "title" metadata field.
	Date time.Time // the date metadata field.
	Tags []string // The tags field in the metadata. It must be an array.
	Content string // Raw markdown content
	Params map[string]any // Fields in the YAML metadata part, except the title, public, date and tags.
	OutLinks []string // The list of nodes this node links to. (Their .File values)
	InLinks []string //The list of nodes contains a link to this node. (Their .File values)
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
					nodeinfo.File=strings.TrimPrefix(npath, notesPath); nodeinfo.Content=""
					// Go maps are not safe for concurrent writes
					nmu.Lock(); servedNodes[nodeinfo.File] = &nodeinfo; nmu.Unlock()
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

	SetInLinks()
	log.Println(len(servedNodes), "nodes are loaded (in", time.Since(startTime).Milliseconds(),"ms)")
	log.Println(len(servedAttachments),"attachments will be served.")
}

func loadNode(relPath string){
	oldNode,ok := servedNodes[relPath]
	if ok || oldNode != nil {
		// First delete the attachments if only one node (this one) has a link to them. As they can be changed when the content is changed. We will reload the new attachments in getFileInfo()
		for _,attachment := range oldNode.Attachments {
			if servedAttachments[attachment] <= 1 {delete(servedAttachments, attachment)}
		}
	}

	absPath := filepath.Join(notesPath,relPath)
	nodeInfo,err := getNodeInfo(absPath, true); if err != nil {log.Println("Error while reloading the node: ",err.Error())}
	nodeInfo.File=relPath; nodeInfo.Content=""

	if !isServed(nodeInfo.Public) {return}

	// Set the inlinks of this node, made from other nodes.
	// This is slow as we have to look for every node's outlinks in the map.
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

func getNodeInfo(filename string, newNode bool) (nodeinfo Node, err error) {
	data, err := os.ReadFile(filename); if err != nil {return nodeinfo, err};
	seq := bytes.SplitSeq(data, []byte("\n"))

	var inMeta bool
	var inExcBlock bool
	var metaBuf bytes.Buffer
	var contentBuf bytes.Buffer
	var tagMap = make(map[string]struct{})
	var linkMap = make(map[string]struct{})
	nodeinfo.Title = filepath.Base(filename)

	for line := range seq {
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
	if !newNode {nodeinfo.Content = strings.TrimSuffix(contentBuf.String(), "\n")}

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

func SetInLinks(){
	for _, fnode := range servedNodes {
		for _, outLink := range fnode.OutLinks {
			// If outLink of the fnode exists in the served files
			if outlinkNode, ok := servedNodes[outLink]; ok && outlinkNode.File != "" {
				// update the outLink node's inlinks and add fnode's MapKey
				outlinkNode.InLinks = append(outlinkNode.InLinks, fnode.File)
			}
		}
	}
}
