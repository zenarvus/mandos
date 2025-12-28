package main

import (
	"database/sql"; "fmt"; "io/fs"; "log"; "os"; "path/filepath"; "strings"; "time";
	_ "github.com/mattn/go-sqlite3"
	_ "github.com/knaka/go-sqlite3-fts5"
)

var DB *sql.DB

// Deleting the database files and recreating them is necessary. Because, for example:
// getNodeInfo function extracts the outlinks and attachments based on the ONLY_PUBLIC option (using isServed function) (Some lines can be excluded). And the getNodeInfo function is used inside upsertNodes.
// We can't use a column named "public" to determine if we are going to serve the node, because of this:
// If its set to ONLY_PUBLIC=yes at first, links in the excluded lines are ignored in the nodes. outlinks and attachments will not contain these.
// When its switched to ONLY_PUBLIC=no, excluded lines should not be excluded, however, we did not insert the links in the excluded lines to the database.
// If its set to ONLY_PUBLIC=no at first, links in the excluded lines are also inserted in the outlinks and attachments tables.
// When its switched to ONLY_PUBLIC=yes, we should exclude the links inside the excluded lines. However, they are in the table and queries will fetch them.
func syncMarker(cacheDir, filename, envValue, disableValue string) (change bool) {
	markerPath := filepath.Join(cacheDir, filename)
	_, err := os.Stat(markerPath)
	markerExists := (err == nil)
	wantsDisabled := envValue == disableValue
	// If (marker doesn't exist but the option is wanted) OR (marker exists but the option is not wanted)
	if (!markerExists && !wantsDisabled) || (markerExists && wantsDisabled) {
		// If the database exists
		if _,err = os.Stat(filepath.Join(cacheDir, "mandos.db")); err == nil {
			fmt.Printf("%s variable has changed. The database will be regenerated.\n", strings.ToUpper(filename))
		}
		if !markerExists { os.WriteFile(markerPath, []byte{}, 0644)
		} else { os.Remove(markerPath) }
		return true // Signal that a change happened
	}
	return false
}
func checkDatabaseConsistency(cacheDir string) {
	change1 := syncMarker(cacheDir, "only_public", getEnvValue("ONLY_PUBLIC"), "no")
	change2 := syncMarker(cacheDir, "content_search", getEnvValue("CONTENT_SEARCH"), "false")
	if change1 || change2 {
		os.Remove(filepath.Join(cacheDir, "mandos.db"))
		os.Remove(filepath.Join(cacheDir, "mandos.db-shm"))
		os.Remove(filepath.Join(cacheDir, "mandos.db-wal"))
	}
}

func InitDB() {
	var err error

	err = os.MkdirAll(getEnvValue("CACHE_FOLDER"), 0755); if err!=nil {log.Fatalln("Cache dir could not be created.", err)}

	checkDatabaseConsistency(getEnvValue("CACHE_FOLDER"))

	// Open (creates file if not exists)
	DB, err = sql.Open("sqlite3", "file:"+filepath.Join(getEnvValue("CACHE_FOLDER"),"mandos.db"))
	if err != nil { log.Fatal(err) }
	// Ensure connection is alive
	if err := DB.Ping(); err != nil { log.Fatal(err) }
	// Optional pragmas for performance
	_, _ = DB.Exec("PRAGMA journal_mode=WAL;") // Enable parallel reading on writes.
	_, _ = DB.Exec("PRAGMA synchronous=NORMAL;")
    _, _ = DB.Exec("PRAGMA foreign_keys = ON;") // Enable foreign keys.
	// Create tables if they don't exist
	if err := ensureSchema(DB); err != nil { log.Fatal(err) }
}
func ensureSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil { return err }
	defer tx.Rollback()

	// Nodes: file is text (filepath), mtime as INTEGER, date as INTEGER (unix seconds), title TEXT
	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS nodes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		file TEXT UNIQUE,
		mtime INTEGER NOT NULL,
		date  INTEGER,
		title TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_node_file ON nodes(file);
	CREATE INDEX IF NOT EXISTS idx_node_date ON nodes(date);
	`)
	if err != nil { return err }

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS outlinks (
		"from" TEXT NOT NULL,
		"to"   TEXT NOT NULL,
		PRIMARY KEY ("from", "to"),
		FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_outlink_to ON outlinks("to");
	`)
	if err != nil { return err }

	_, err = tx.Exec(`CREATE TABLE IF NOT EXISTS attachments (
		"from" TEXT NOT NULL,
		file TEXT NOT NULL,
		PRIMARY KEY ("from", file)
		FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
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
		FOREIGN KEY ("from") REFERENCES nodes(file) ON DELETE CASCADE
	) WITHOUT ROWID;
	CREATE INDEX IF NOT EXISTS idx_params_key_val_from ON params(key, value, "from");
	CREATE INDEX IF NOT EXISTS idx_params_from ON params("from");
	`)
	if err != nil { return err }

	// FTS5 Virtual Table for content searching
	if getEnvValue("CONTENT_SEARCH") == "true" {
		_, err = tx.Exec(`CREATE VIRTUAL TABLE IF NOT EXISTS nodes_fts USING fts5(
			title, content,
			content='', contentless_delete=1,
			tokenize="unicode61 remove_diacritics 2 tokenchars '#'"
		);`)
		if err != nil { return err }
		// Sync On Delete (Like ON DELETE CASCADE)
		_, err = tx.Exec(`CREATE TRIGGER IF NOT EXISTS nodes_ad AFTER DELETE ON nodes BEGIN
  			DELETE FROM nodes_fts WHERE rowid = old.id;
		END;`)
		if err != nil { return err }
		// We first delete, then insert. No need for update sync.
	}

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

	rows, err := DB.Query(`SELECT file, mtime FROM nodes;`)
	if err != nil { log.Fatalln(err) }
	defer rows.Close()

	for rows.Next() {
		var file string; var mtime int64
		if err := rows.Scan(&file, &mtime); err != nil { log.Fatalln(err) }
		sqlNodeMtimes[file] = mtime
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

		// TODO: This also skips the directories named "static" and "mandos" that are not in the root. Fix that.
		}else if d.IsDir() && (d.Name() == "static" || d.Name() == "mandos") { return filepath.SkipDir }
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


func deleteNodes(nodeIds []string) {
    if len(nodeIds) == 0 { return }

    tx, err := DB.Begin()
    if err != nil { log.Println(err); return }
    defer tx.Rollback()

    delNodes, _ := tx.Prepare(`DELETE FROM nodes WHERE file = ?`)
    defer delNodes.Close()

    for _, id := range nodeIds {
		delNodes.Exec(id);
		// Remove the node from the cache.
		nodeCache.Delete(id)
	}
    tx.Commit()
}

func upsertNodes(nodeIdMTimeMap map[string]int64) (upserted int) {
	if len(nodeIdMTimeMap) == 0 {return 0}

	nodes := bulkNodeInfo(nodeIdMTimeMap)

	// Start the transaction (Atomic update)
	tx, err := DB.Begin()
	if err != nil { log.Println("Failed to begin transaction:", err); return 0}
	// Safety: If we panic or return early, rollback changes
	defer tx.Rollback()

	// PREPARE STATEMENTS
	delNodes, _ := tx.Prepare(`DELETE FROM nodes WHERE file = ?`) // For cleaning the non-public nodes and old attachments, params and outlinks that does not exist anymore.
	defer delNodes.Close()
	
	stmtNode, _ := tx.Prepare(`INSERT INTO nodes (file, mtime, date, title) VALUES (?, ?, ?, ?)`)
	defer stmtNode.Close()

	var stmtNodeFTS *sql.Stmt
	if getEnvValue("CONTENT_SEARCH")=="true"{
		stmtNodeFTS, _ = tx.Prepare(`INSERT INTO nodes_fts (rowid, title, content) VALUES (?, ?, ?)`)
		defer stmtNodeFTS.Close()
	}
	
	stmtLink, _ := tx.Prepare(`INSERT INTO outlinks ("from", "to") VALUES (?, ?)`)
	defer stmtLink.Close()
	
	stmtAtt, _ := tx.Prepare(`INSERT INTO attachments ("from", "file") VALUES (?, ?)`)
	defer stmtAtt.Close()
	
	// Using INSERT OR IGNORE to handle potential duplicate params/tags gracefully
	stmtParam, _ := tx.Prepare(`INSERT OR IGNORE INTO params ("from", "key", "value") VALUES (?, ?, ?)`)
	defer stmtParam.Close()

	// PROCESS THE BATCH
	for _, node := range nodes {
		// Delete existing node.
		if _, err := delNodes.Exec(node.File); err != nil { log.Println("Error deleting node:", node.File, err) }
	
		// Skip the private nodes.
		if !isServed(node.Public){continue}

		// Update the node in the cache, without moving it to forward.
		nodeCache.Update(node.File, node)

		// Insert the node
		result, err := stmtNode.Exec(node.File, nodeIdMTimeMap[node.File], node.Date, node.Title);
		// If it gives an error, skip inserting things related to this node completely.
		if err != nil { log.Println("Error inserting node:", node.File, err); continue }

		// Insert the index of the node content.
		if getEnvValue("CONTENT_SEARCH")=="true" {
			newNodeRowId, err := result.LastInsertId()
			if err != nil{log.Println("Error while getting node's last insert id:", node.File, err)}

			_,err = stmtNodeFTS.Exec(newNodeRowId, node.Title, node.Content)
			if err!=nil{log.Println("Error while inserting index of the node content:",node.File, err);}
		}

		// Insert Outlinks
		for _, target := range node.OutLinks { _,err := stmtLink.Exec(node.File, target); if err!=nil{log.Println(node.File, target, err)} }
		// Insert Attachments
		for _, att := range node.Attachments { _,err := stmtAtt.Exec(node.File, att); if err!=nil{log.Println(node.File, att, err)} }

		// Insert Params. Params is map[string]any, but values can only be string or []string
		for key, val := range node.Params {
			switch v := val.(type) {
			case string: stmtParam.Exec(node.File, key, v)

			case []string: for _, subVal := range v { stmtParam.Exec(node.File, key, subVal) }

			default:
				// Handle other types (int, bool) if your metadata supports them
				// fmt.Sprint(v) is a safe fallback
				stmtParam.Exec(node.File, key, fmt.Sprint(val))
			}
		}
		upserted++
	}
	// Commit everything at once
	if err := tx.Commit(); err != nil { log.Println("Failed to commit transaction:", err) }
	return upserted
}

// Execute the queryStr with queryVals values, then return the rows in []map[string]any where key is the column name and value is the column value.
func GetRows(queryStr string, queryVals []any) (returnData []map[string]any) {
	// Prefer the cached data.
	returnData, exists := queryCache.Get(GetQueryKey(queryStr, queryVals...))
	if exists {return returnData}

	rows, err := DB.Query(queryStr, queryVals...)
	if err!=nil{log.Println(err); return returnData}
	defer rows.Close()
	// Get the column names
	columns,err := rows.Columns()
	if err!=nil{log.Println("Columns error:",err); return returnData}

	for rows.Next() {
		// Prepare a slice of 'any' to hold the data and a slice of pointers to those 'any'
		values := make([]any, len(columns))
		valuePointers := make([]any, len(columns))
		for i := range values { valuePointers[i] = &values[i] }
		// Scan columns in the row and set their values to values slice.
		if err := rows.Scan(valuePointers...); err != nil {
			log.Println("failed to scan node row: %w", err); return returnData
		}
		rowMap := make(map[string]any)
		for i, colName := range columns {
			val := values[i]
			// SQLite returns texts as []byte. We need to convert them to strings.
			if b, ok := val.([]byte); ok { rowMap[colName] = string(b)
			} else { rowMap[colName] = val }
		}
		returnData = append(returnData, rowMap)
	}

	// Cache the returned data.
	queryCache.Set(GetQueryKey(queryStr, queryVals...), returnData, time.Second*10)

	return returnData
}
