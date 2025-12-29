package main

import (
	"bytes"; "fmt"; "os"; "path/filepath"; "strings"; "time"; "regexp"; "unicode/utf8"
	"gopkg.in/yaml.v3";
)
// Key is the relative file location starting with slash, considering notesPath as root.
type Node struct {
	File string // The same with the key. Only used in templates, otherwise empty.
	Public bool // Is the file is public?
	Title string // The last H1 heading or the "title" metadata field.
	Date int64 // the date metadata field.
	Content string // Raw markdown content. Only used in templates.
	Params map[string]any // Fields in the YAML metadata part, except the title, public, and date. Must be []string or string
	OutLinks []string // The list of nodes this node links to. (Their .File values)
	Attachments []string // Local non-markdown links in a node.
}

var mdLinkRe = regexp.MustCompile(`\]\(/([^)?#]*)[^)]*\)`) // Extract internal markdown links. Do not capture after ? or #
var htmlSrcRe = regexp.MustCompile(`<[^>]+src="/([^"?#]+)[^>]`) // Extract internal html links inside src. Do not capture after ? or #
func getNodeInfo(relPath string, onlyContent bool) (nodeinfo Node, err error) {

	absPath := SafeJoin(notesPath, relPath)
	if absPath==""{return nodeinfo,err}

	data, err := os.ReadFile(absPath); if err != nil {return nodeinfo, err};

	var inMeta bool
	var inExcBlock bool
	var metaBuf bytes.Buffer
	var contentBuf bytes.Buffer
	var linkMap = make(map[string]struct{})
	nodeinfo.Title = relPath
	var gotTitle bool

	nodeinfo.File = relPath

	for line := range bytes.SplitSeq(data, []byte("\n")) {
		// Exclude lines if ONLY_PUBLIC != "no"
		if onlyPublic != "no" {
			if !inExcBlock && bytes.Contains(line, []byte("<!--exc:start-->")){inExcBlock=true; continue}
			if inExcBlock && bytes.Contains(line, []byte("<!--exc:end-->")){inExcBlock=false; continue}
			if inExcBlock || bytes.Contains(line, []byte("<!--exc-->")){continue}
		}

		// Run the link finding regexp if the line contains "](" or "src="
		if !onlyContent && bytes.Contains(line, []byte("](")){
			for _,match := range mdLinkRe.FindAllSubmatch(line,-1) {
				// The link should start with a slash and the link path must consider notesPath as root. It should not be an absolute path or relative path inside the file system.
				linkMap[filepath.Join("/", string(match[1]))] = struct{}{}
			}
		}
		if !onlyContent && bytes.Contains(line, []byte("src=")) {
			for _,match := range htmlSrcRe.FindAllSubmatch(line,-1) {
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
			if !onlyContent { metaBuf.Write(line); metaBuf.WriteByte('\n') }

			continue
		}

		// Extract the title
		if !onlyContent && !gotTitle && bytes.HasPrefix(line, []byte("# ")){
			nodeinfo.Title = strings.TrimPrefix(string(line), "# "); gotTitle = true;
		}

		// Write the lines to the content buffer
		contentBuf.Write(line)
        contentBuf.WriteByte('\n')
	}

	nodeinfo.Content = strings.TrimSuffix(contentBuf.String(), "\n")

	// Add the links to the fileinfo.Attachments or fileinfo.Outlinks (Only if they exist in the filesystem)
	for link := range linkMap{
		// If its a markdown file, add to the outlinks.
		if strings.HasSuffix(link, ".md"){
			nodeinfo.OutLinks = append(nodeinfo.OutLinks, link)

		// If its a non-markdown file, add to the attachments
		} else { nodeinfo.Attachments = append(nodeinfo.Attachments, link) }
	}

	// Parse the YAML metadata to fileinfo.Params
    if metaBuf.Len() != 0 {
        if err := yaml.Unmarshal(metaBuf.Bytes(), &nodeinfo.Params); err != nil {
            return nodeinfo, fmt.Errorf("%s: %w", relPath, err)
        }
    }

	if !onlyContent {
		// Get the public field in the metadata
		if isPublic,ok := nodeinfo.Params["public"].(bool); ok && isPublic {nodeinfo.Public = isPublic; delete(nodeinfo.Params, "public")}
		// Get the date of the node from the metadata
		if yamlDate,ok := nodeinfo.Params["date"].(time.Time); ok {nodeinfo.Date=yamlDate.Unix(); delete(nodeinfo.Params,"date")}
		// Prefer the metadata title over the first header
		if mTitle,ok := nodeinfo.Params["title"].(string); ok && mTitle != "" { nodeinfo.Title = mTitle; delete(nodeinfo.Params,"title") }
	}

	return nodeinfo, nil
}

func GetNodeContent(relPath string) string {
	// Prefer cache.
	nodeinfo, exists := nodeCache.Get(relPath)
	if !exists {
		var err error
		nodeinfo, err = getNodeInfo(relPath, true)
		if err != nil {return err.Error()}
		nodeCache.Put(relPath, nodeinfo) // Cache the node to the memory.
	}
	return nodeinfo.Content
}

var fts5Query = regexp.MustCompile(`"([^"]+)"|(\w+)`)
// GetContentMatch extracts a highlighted snippet from content. 
// It looks for a line containing the most query tokens. If a line contains all tokens, 
// it returns a windowed snippet around the match. Otherwise, it falls back to 
// the best partial match found. It handles UTF-8 safely and avoids per-line allocations.
func GetContentMatch(content string, searchQuery string, window int) string {
	if content == "" || searchQuery == "" { return "" }

	// Pre-process tokens
	matches := fts5Query.FindAllStringSubmatch(searchQuery, -1)
	var uniqueTokens []string

	seen := make(map[string]bool)
	// m[1] is the string between quotes, and m[2] is the unquoted word.
	for _, m := range matches {
		term := m[1] 
		// If m[1] is an empty string, it means that the unquoted word (m[2]) is matched.
		if term == "" {
			term = m[2]
			if len(term) < 3 { continue } // Skip the term if its too short.
		}
		// If the term has already seen, or its one of the FTS5 query operation strings, skip it.
		if term == "AND" || term == "OR" || term == "NOT" || seen[term] { continue }

		term = strings.ToLower(term)
		seen[term] = true

		uniqueTokens = append(uniqueTokens, term)
	}

	if len(uniqueTokens) == 0 { return "" }

	var bestLine string // The line containing the most tokens.
	var bestFirstByte = -1
	var bestLastByte = -1
	var maxTokensMatched = 0 // Maximum amount of tokens matched in the whole content.

	for line := range strings.SplitSeq(content, "\n") {
		// We use a lower-case version for searching, but keep the original for the snippet.
		lowerLine := strings.ToLower(line)
		
		// Start position of the first token in the line, and end position of the last token in the line.
		firstBytePos, lastBytePos := -1, -1
		tokensInThisLine := 0

		for _, token := range uniqueTokens {
			// Find the byte offset of the token
			idx := strings.Index(lowerLine, token)
			if idx == -1 { continue } // If the token does not exists in the line, skip the token.
			tokensInThisLine++ // If its found, increase token count in the line.
			
			// Track the outer boundaries (in bytes) of all found tokens in this line

			// If this is the first token matched, or this token comes before the previously matched token, make the firstBytePos = idx
			if firstBytePos == -1 || idx < firstBytePos { firstBytePos = idx }

			endIdx := idx + len(token) // Get the end position of the token in the line.

			// If this token's end position is later than the previous token's end, make lastBytePos = endIdx
			if endIdx > lastBytePos { lastBytePos = endIdx }
		}

		// Update the "Best Match"
		// If we found all tokens, we can stop early.
		// Otherwise, we keep track of the line that had the most matches.
		if tokensInThisLine > maxTokensMatched {
			maxTokensMatched = tokensInThisLine
			bestLine = line
			bestFirstByte = firstBytePos
			bestLastByte = lastBytePos

			if tokensInThisLine == len(uniqueTokens) { break }
		}
	}

	// If nothing matched in the content, return an empty string.
	if bestLine == "" { return "" }

	// Convert byte offsets to rune offsets by only scanning the necessary parts
	// startRune is the count of runes before the match
	startRuneIdx := utf8.RuneCountInString(bestLine[:bestFirstByte])
	// matchRuneLen is the count of runes within the matched portion
	matchRuneLen := utf8.RuneCountInString(bestLine[bestFirstByte:bestLastByte])
	
	runes := []rune(bestLine)
	totalRunes := len(runes)

	// Calculate window boundaries in rune-space to prevent splitting a character
	startBound := max(0, startRuneIdx - window)
	endBound := min(totalRunes, startRuneIdx + matchRuneLen+window)

	// Match start relative to our slice
	relMatchStart := startRuneIdx - startBound
	relMatchEnd := relMatchStart + matchRuneLen

	// Build the snippet
	// We slice the rune array and convert only that small segment to a string.
	snippetRunes := runes[startBound:endBound]
	
	return fmt.Sprintf("...%s<b>%s</b>%s...",
		string(snippetRunes[:relMatchStart]),
		string(snippetRunes[relMatchStart:relMatchEnd]),
		string(snippetRunes[relMatchEnd:]),
	)
}
