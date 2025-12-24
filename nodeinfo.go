package main

import (
	"bytes"; "fmt"; "os"; "path/filepath"; "strings"; "time"
	"gopkg.in/yaml.v3"
)
// Key is the relative file location starting with slash, considering notesPath as root.
type Node struct {
	File *string // The same with the key. Only used in templates, otherwise empty.
	Public bool // Is the file is public?
	Title string // The last H1 heading or the "title" metadata field.
	Date time.Time // the date metadata field.
	Tags []string // The tags field in the metadata. It must be an array.
	Content string // Raw markdown content. Only used in templates.
	Params map[string]any // Fields in the YAML metadata part, except the title, public, date and tags. Must be []string or string
	OutLinks []string // The list of nodes this node links to. (Their .File values)
	Attachments []string // Local non-markdown links in a node.
}

func getNodeInfo(nodeId string) (nodeinfo Node, err error) {
	data, err := os.ReadFile(filepath.Join(notesPath, nodeId)); if err != nil {return nodeinfo, err};

	var inMeta bool
	var inExcBlock bool
	var metaBuf bytes.Buffer
	var contentBuf bytes.Buffer
	var tagMap = make(map[string]struct{})
	var linkMap = make(map[string]struct{})
	nodeinfo.Title = nodeId
	var gotTitle bool

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
		if !gotTitle && bytes.HasPrefix(line, []byte("# ")){ nodeinfo.Title = strings.TrimPrefix(string(line), "# "); gotTitle = true; }

		// Write the lines to the content buffer
		contentBuf.Write(line)
        contentBuf.WriteByte('\n')
	}

	nodeinfo.Content = strings.TrimSuffix(contentBuf.String(), "\n")

	// Add the tags in the tagMap to fileinfo.Tags
	for tag := range tagMap{nodeinfo.Tags=append(nodeinfo.Tags, tag)}

	// Add the links to the fileinfo.Attachments or fileinfo.Outlinks (Only if they exist in the filesystem)
	for link := range linkMap{
		if _,err := os.Stat(notesPath+link); err != nil {continue}
		// If its a markdown file, add to the outlinks.
		if strings.HasSuffix(link, ".md"){

			nodeinfo.OutLinks = append(nodeinfo.OutLinks, link)
		// If its a non-markdown file, add to the attachments
		} else { nodeinfo.Attachments = append(nodeinfo.Attachments, link) }
	}

	// Parse the YAML metadata to fileinfo.Params
    if metaBuf.Len() != 0 {
        if err := yaml.Unmarshal(metaBuf.Bytes(), &nodeinfo.Params); err != nil {
            return nodeinfo, fmt.Errorf("%s: %w", nodeId, err)
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
