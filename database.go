package main

import (
	"database/sql"
	"fmt"
	"sync"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB
func InitDB() {
	var err error

	userCache, err := os.UserCacheDir(); if err!=nil{log.Fatalln("Cache dir could not be determined.", err)}
	cacheDir := filepath.Join(userCache,"mandos")
	err = os.MkdirAll(cacheDir, 0755); if err!=nil {log.Fatalln("Cache dir could not be created.", err)}

	_, err = os.Stat(filepath.Join(cacheDir,"only_public"))
	// If err!=nil (file does not exists, the server was serving everything), however the ONLY_PUBLIC environment variable is not "no", delete the database to restructure it for ONLY_PUBLIC nodes.
	if err!= nil && getEnvValue("ONLY_PUBLIC")!="no"{
		os.WriteFile(filepath.Join(cacheDir,"only_public"), []byte{}, 0644)
		// If the database exists
		if _,err = os.Stat(filepath.Join(cacheDir, "mandos.db")); err == nil {
			fmt.Println("ONLY_PUBLIC variable has changed. The database will be regenerated.")
			os.Remove(filepath.Join(cacheDir, "mandos.db"))
			os.Remove(filepath.Join(cacheDir, "mandos.db-shm"))
			os.Remove(filepath.Join(cacheDir, "mandos.db-wal"))
		}

	// If the file named only_public exists, (the server was serving only public nodes), however the ONLY_PUBLIC environment variable is set to "no", delete the database to restructure it for serving everything.
	}else if err==nil && getEnvValue("ONLY_PUBLIC")=="no"{
		// If serving only public nodes, create a file named only_public.
		fmt.Println("ONLY_PUBLIC variable has changed. The database will be regenerated.")
		os.Remove(filepath.Join(cacheDir, "mandos.db"))
		os.Remove(filepath.Join(cacheDir, "mandos.db-shm"))
		os.Remove(filepath.Join(cacheDir, "mandos.db-wal"))
		os.Remove(filepath.Join(cacheDir, "only_public"))
	}
	// The above code is necessary because getNodeInfo function extracts the outlinks and attachments based on the ONLY_PUBLIC option (using isServed function) (Some lines can be excluded). And the getNodeInfo function is used inside upsertNodes.
	// We can't use a column named "public" to determine if we are going to serve the node, because of this:
	// If its set to ONLY_PUBLIC=yes at first, links in the excluded lines are ignored in the nodes. outlinks and attachments will not contain these.
	// When its switched to ONLY_PUBLIC=no, excluded lines should not be excluded, however, we did not insert the links in the excluded lines to the database.
	// If its set to ONLY_PUBLIC=no at first, links in the excluded lines are also inserted in the outlinks and attachments tables.
	// When its switched to ONLY_PUBLIC=yes, we should exclude the links inside the excluded lines. However, they are in the table and queries will fetch them.

	// Open (creates file if not exists)
	DB, err = sql.Open("sqlite3", "file:"+filepath.Join(cacheDir,"mandos.db"))
	if err != nil { log.Fatal(err) }
	// Ensure connection is alive
	if err := DB.Ping(); err != nil { log.Fatal(err) }
	// Optional pragmas for performance
	_, _ = DB.Exec("PRAGMA journal_mode=WAL;")
	_, _ = DB.Exec("PRAGMA synchronous=NORMAL;")
    _, _ = DB.Exec("PRAGMA foreign_keys = ON;")
	// Create tables if they don't exist
	if err := ensureSchema(DB); err != nil { log.Fatal(err) }
}
func ensureSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	// Nodes: id is text (filepath), mtime as INTEGER, date as INTEGER (unix seconds), title TEXT
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS nodes (
		id    TEXT PRIMARY KEY,
		mtime INTEGER NOT NULL,
		date  INTEGER,
		title TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_node_date ON nodes(date);
	`)
	if err != nil { return err }

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS outlinks (
		"from" TEXT NOT NULL,
		"to"   TEXT NOT NULL,
		PRIMARY KEY ("from", "to"),
		FOREIGN KEY ("from") REFERENCES nodes(id) ON DELETE CASCADE
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_outlink_to ON outlinks("to");
	`)
	if err != nil { return err }

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS attachments (
		"from" TEXT NOT NULL,
		file TEXT NOT NULL,
		PRIMARY KEY ("from", file)
		FOREIGN KEY ("from") REFERENCES nodes(id) ON DELETE CASCADE
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_attachment_file ON attachments(file);
	`)
	if err != nil { return err }

	// Params: one row per (from, key, value). Unique constraint prevents duplicates.
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS params (
		"from"  TEXT NOT NULL,
		key   TEXT NOT NULL,
		value TEXT NOT NULL,
		PRIMARY KEY ("from", key, value)
		FOREIGN KEY ("from") REFERENCES nodes(id) ON DELETE CASCADE
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_params_key_val_from ON params(key, value, "from");
	CREATE INDEX IF NOT EXISTS idx_params_from ON params("from");
	`)
	if err != nil { return err }
	return tx.Commit()
}

// Modification times of the nodes in the db.
// key: path of the markdown node, considering notesPath as root
// value: modification time of the node
var sqlNodeMtimes = make(map[string]int64)
// Synchronize the filesystem with the database. Update modified nodes, remove deleted nodes and add new nodes.
func initialSyncWithDB() {
	fmt.Println("Syncing the database with the filesystem.")

	syncStartTime := time.Now()

	rows, err := DB.Query(`SELECT id, mtime FROM nodes;`)
	if err != nil { log.Fatalln(err) }
	defer rows.Close()

	for rows.Next() {
		var id string; var mtime int64
		if err := rows.Scan(&id, &mtime); err != nil { log.Fatalln(err) }
		sqlNodeMtimes[id] = mtime
	}

	var newNodes = make(map[string]int64) // New and modified nodes.

	err = filepath.WalkDir(notesPath, func(npath string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		fileName := filepath.Base(d.Name())
		// Get only the non-hidden markdown files
		if !d.IsDir() && strings.HasSuffix(fileName, ".md") && !strings.HasPrefix(fileName,".") {
					
			relPath := strings.TrimPrefix(npath, notesPath)
			fileinf,err := d.Info(); if err!=nil{ log.Println(relPath, err) }
			mTime := fileinf.ModTime().Unix()

			// Add to the newNodes table if it does not exists in the sqlNodeMtimes map or has a modified mtime
			if sqlNodeMtimes[relPath] == 0 || (sqlNodeMtimes[relPath] != 0 && mTime != sqlNodeMtimes[relPath]) { newNodes[relPath]=mTime }

			// Delete the node from the sqlNodeMtimes map if it exists in the filesystem. The remaining will be the deleted nodes.
			if sqlNodeMtimes[relPath] != 0 { delete(sqlNodeMtimes, relPath) }

		// TODO: This also skips the directories named "static" that are not in the root. Fix that.
		}else if d.IsDir() && d.Name() == "static" { return filepath.SkipDir }
		return nil
	})
	if err != nil {fmt.Println("Error walking the path:", err)}

	// The remaining sqlNodeMtimes fields are deleted ones. If they were exist in the filesystem, the code above would remove them from the map.
	// Delete them from the database.
	var deletedNodes []string
	for deletedId := range sqlNodeMtimes {deletedNodes = append(deletedNodes, deletedId)}
	deleteNodes(deletedNodes)

	fmt.Println(len(deletedNodes), "node(s) are deleted from the database.")

	// Add new nodes and update updated
	fmt.Println(upsertNodes(newNodes), "node(s) are upserted in the database.")

	fmt.Printf("Database synchronization is completed in %v ms\n", time.Since(syncStartTime).Milliseconds())
}


// Done, I do not need to touch it anymore, until a change is needed.
func deleteNodes(nodeIds []string) {
    if len(nodeIds) == 0 { return }

    tx, err := DB.Begin()
    if err != nil { log.Println(err); return }
    defer tx.Rollback()

    delNodes, _ := tx.Prepare(`DELETE FROM nodes WHERE "id" = ?`)
    defer delNodes.Close()

    for _, id := range nodeIds { delNodes.Exec(id); }
    tx.Commit()
}

func bulkNodeInfo(nodeIdMTimeMap map[string]int64) ([]Node) {
	var nodes = make([]Node, 0, len(nodeIdMTimeMap))
	var multiThread bool
	if len(nodeIdMTimeMap) > 100 {multiThread=true}

	if multiThread {
		pathsCh := make(chan string, 256)
		var resultsCh = make(chan Node, len(nodeIdMTimeMap))
		var wg sync.WaitGroup
		for i:=0; i <  runtime.NumCPU(); i++ {
			wg.Add(1)
			go func(){
				defer wg.Done()

				for npath := range pathsCh {
					nodeinfo, err := getNodeInfo(npath); if err != nil { fmt.Println("Error processing files:", err) }
					nodeinfo.Content="";

					if isServed(nodeinfo.Public) {
						relPath := strings.TrimPrefix(npath, notesPath);
						nodeinfo.File = &relPath
						// Go maps are not safe for concurrent writes
						resultsCh <- nodeinfo
					}
				}

			}()
		}
		for npath := range nodeIdMTimeMap { pathsCh <- npath }

		close(pathsCh)
		wg.Wait()
		
		close(resultsCh)
		for node := range resultsCh {nodes = append(nodes, node)}

	} else {
		for npath := range nodeIdMTimeMap {
			nodeinfo, err := getNodeInfo(npath); if err != nil { fmt.Println("Error processing files:", err) }
			nodeinfo.Content="";

			if isServed(nodeinfo.Public) {
				relPath := strings.TrimPrefix(npath, notesPath);
				nodeinfo.File = &relPath
				nodes = append(nodes, nodeinfo);
			}
		}
	}
	return nodes
}

// CONSIDER?: Instead of getting node infos while the database is locked, first get the infos multi-threaded and use them in here (This will increase memory usage for large nodes in the first run.)
func upsertNodes(nodeIdMTimeMap map[string]int64) (upserted int) {
	if len(nodeIdMTimeMap) == 0 { return 0}

	nodes := bulkNodeInfo(nodeIdMTimeMap)

	// 1. Start the transaction (Atomic update)
	tx, err := DB.Begin()
	if err != nil { log.Println("Failed to begin transaction:", err); return 0}
	// Safety: If we panic or return early, rollback changes
	defer tx.Rollback()

	// 2. PREPARE STATEMENTS
	// We compile the SQL once, then reuse it thousands of times.
	// using "INSERT OR REPLACE" or "INSERT OR IGNORE" handles edge cases nicely.
	delNodes, _ := tx.Prepare(`DELETE FROM nodes WHERE id = ?`)
	
	stmtNode, _ := tx.Prepare(`
		INSERT INTO nodes (id, mtime, date, title) 
		VALUES (?, ?, ?, ?)
	`)
	
	stmtLink, _ := tx.Prepare(`
		INSERT INTO outlinks ("from", "to") 
		VALUES (?, ?)
	`)
	
	stmtAtt, _ := tx.Prepare(`
		INSERT INTO attachments ("from", "file") 
		VALUES (?, ?)
	`)
	
	// Using INSERT OR IGNORE to handle potential duplicate params/tags gracefully
	stmtParam, _ := tx.Prepare(`
		INSERT OR IGNORE INTO params ("from", "key", "value") 
		VALUES (?, ?, ?)
	`)

	// Close statements when function exits
	defer delNodes.Close()
	defer stmtNode.Close()
	defer stmtLink.Close()
	defer stmtAtt.Close()
	defer stmtParam.Close()

	// 3. PROCESS THE BATCH
	for _, node := range nodes {
		// A. Delete existing node.
		if _, err := delNodes.Exec(node.File); err != nil { log.Println("Error deleting node:", node.File, err) }
		
		if !isServed(node.Public){continue}

		// C. Insert Node
		// Ensure we convert time.Time to Unix Integer
		date := node.Date.Unix()
		if _, err := stmtNode.Exec(node.File, nodeIdMTimeMap[*node.File], date, node.Title); err != nil {
			log.Println("Error inserting node:", node.File, err)
		}

		// D. Insert Outlinks
		for _, target := range node.OutLinks { _,err := stmtLink.Exec(node.File, target); if err!=nil{log.Println(node.File, target, err)} }

		// E. Insert Attachments
		for _, att := range node.Attachments { _,err := stmtAtt.Exec(node.File, att); if err!=nil{log.Println(node.File, att, err)} }

		// F. Insert Tags (Saving them as params with key="tags")
		for _, tag := range node.Tags { _,err:= stmtParam.Exec(node.File, "tags", tag); if err!=nil{log.Println(node.File, tag, err)} }

		// G. Insert Params
		// Params is map[string]any, but values can be string or []string
		for key, val := range node.Params {
			switch v := val.(type) {
			case string:
				stmtParam.Exec(node.File, key, v)
			case []string:
				for _, subVal := range v {
					stmtParam.Exec(node.File, key, subVal)
				}
			default:
				// Handle other types (int, bool) if your metadata supports them
				// fmt.Sprint(v) is a safe fallback
				stmtParam.Exec(node.File, key, fmt.Sprint(val))
			}
		}

		upserted++
	}

	// 4. Commit everything at once
	if err := tx.Commit(); err != nil {
		log.Println("Failed to commit transaction:", err)
	}
	return upserted
}

var incParAgg = `ParamsAgg AS (
	SELECT p."from", GROUP_CONCAT(p.key || '=' || p.value, '|||') as params_str
	FROM params AS p
	INNER JOIN TargetNodes tn ON p."from" = tn.id
	GROUP BY p."from"
), `
var incOutAgg = `OutlinksAgg AS (
    SELECT o."from", GROUP_CONCAT(o."to", '|||') as outlinks_str
    FROM outlinks AS o
    INNER JOIN TargetNodes tn ON o."from" = tn.id
    GROUP BY o."from"
), `
var incAttAgg = `AttachmentsAgg AS (
    SELECT a."from", GROUP_CONCAT(a.file, '|||') as attachments_str
    FROM attachments AS a
    INNER JOIN TargetNodes tn ON a."from" = tn.id
    GROUP BY a."from"
), `

var incParSelect = `par.params_str AS params, `
var incOutSelect = `ol.outlinks_str AS outlinks, `
var incAttSelect = `att.attachments_str AS attachments, `

var incParJoin = ` LEFT JOIN ParamsAgg AS par ON tn.id = par."from"`
var incOutJoin = ` LEFT JOIN OutlinksAgg AS ol ON tn.id = ol."from"`
var incAttJoin = ` LEFT JOIN AttachmentsAgg AS att ON tn.id = att."from"`

// Filter should be something like this: JOIN params AS p ON n.id = p."from" WHERE p.key = ? AND p.value = ?
// queryVals should be something like this: []any{'tags', 'moc'}
// inclusions must be an array of boolean values with a length of 3. The values in it determines the values of includeParams, includeOutlinks and includeAttachments respectively.
// orderBy should be one of the fields of the nodes table. (id, mtime, title, date)
// orderType must be ASC OR DESC
func GetNodes(filter string, queryVals []any, inclusions []bool, orderBy, orderType string) (nodes []Node) {

	var includeParams, includeOutlinks, includeAttachments bool
	if len(inclusions) == 3 {
		includeParams = inclusions[0]; includeOutlinks = inclusions[1]; includeAttachments = inclusions[2]
	}

	var aggregation, selection, joins string

	if includeParams {aggregation += incParAgg; selection += incParSelect; joins += incParJoin}
	if includeOutlinks {aggregation += incOutAgg; selection += incOutSelect; joins += incOutJoin}
	if includeAttachments {aggregation += incAttAgg; selection += incAttSelect; joins += incAttJoin}

	if aggregation != "" { aggregation = ", "+strings.TrimSuffix(aggregation, ", ") }
	if selection != "" { selection = ", "+strings.TrimSuffix(selection, ", ") }

	var order string
	if orderBy!="" && orderType != "" {order="ORDER BY tn."+orderBy+" "+orderType}

	query := fmt.Sprintf(`
	WITH TargetNodes AS (
		SELECT n.id, n.date, n.title
		FROM nodes AS n
		%s
	)%s
	SELECT tn.id, tn.date, tn.title%s
	FROM TargetNodes AS tn
	%s
	%s;
	`, filter, aggregation, selection, joins, order)

	rows, err := DB.Query(query, queryVals...)
	if err!=nil{log.Println(err); return nodes}

	for rows.Next() {
		var id string
		var date sql.NullInt64
		var title sql.NullString

		var paramsStr, outlinksStr, attachmentsStr sql.NullString

		scans := []any{&id, &date, &title}
		if includeParams {scans = append(scans, &paramsStr)}
		if includeOutlinks {scans = append(scans, &outlinksStr)}
		if includeAttachments {scans = append(scans, &attachmentsStr)}

		if err := rows.Scan(scans...); err != nil {
			log.Println("failed to scan node row: %w", err); return nodes
		}

		node := Node{
			File: &id,
			Public: true,
			Title: title.String,
			Date: time.Unix(date.Int64,0),
			Content: "",
			OutLinks: strings.Split(outlinksStr.String, "|||"),
			Attachments: strings.Split(attachmentsStr.String, "|||"),
		}	
		var params = make(map[string]any)
		for param := range strings.SplitSeq(paramsStr.String, "|||") {
			keyValue := strings.Split(param,"=")
			if len(keyValue)==2 {
				if keyValue[0]=="tags" { node.Tags=append(node.Tags, keyValue[1])
				}else{ params[keyValue[0]]=keyValue[1] }
			}
		}
		node.Params=params

		nodes = append(nodes, node)
	}
	return nodes
}
