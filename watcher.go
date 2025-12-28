package main
import ("fmt";"log";"os";"path/filepath";"strings";"time"; "github.com/fsnotify/fsnotify")

var waitTime = time.Millisecond * 300
var debounceMutex = make(chan struct{}, 1)
// To debounce per file. (Because watcher can detect two or more separate events on the same file.)
var debounceTimer = make(map[string]*time.Timer)

func scheduleLoad(path string, run func()){
	debounceMutex <- struct{}{}
	defer func() { <-debounceMutex }()
	//fmt.Println("A scheduleLoad request has been made")

	if t := debounceTimer[path]; t != nil {t.Stop()}
	timer := time.AfterFunc(waitTime, func() {
		debounceMutex <- struct{}{}
		//fmt.Println("Only this will be handled")
		delete(debounceTimer, path)
		run()
		<-debounceMutex
	})
	debounceTimer[path]=timer
}

func watchFileChanges() {
	fmt.Println("Watching for file changes.")
	watcher, err := fsnotify.NewWatcher(); if err != nil {log.Fatal(err)}
	defer watcher.Close()
	// Helper function to add a directory and all its subdirectories to the watcher
	addWatchRecursive := func(root string) error {
		// Walk the directory tree
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {return err}
			if info.IsDir(){err:=watcher.Add(path); if err!=nil{return fmt.Errorf("Error adding directory %s to watcher: %w", path, err)}}
			return nil
		})
	}
	err = addWatchRecursive(notesPath)
	if err != nil {log.Fatalf("Failed to watch directories: %v", err)}
	// Listen for events
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {return}
			// If a file is modified, created, or deleted, reload the servedFiles map
			// Ignore the hidden files and folders.
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 &&
			!strings.HasPrefix(filepath.Base(event.Name),"."){
				relPath := strings.TrimPrefix(event.Name, notesPath)
				// If the file was a markdown note
				if strings.HasSuffix(event.Name, ".md") {

					if event.Has(fsnotify.Remove) {
						scheduleLoad(event.Name, func(){
							deleteNodes([]string{relPath})
							log.Println("A node has been deleted: ",relPath)
						})
					}else if event.Has(fsnotify.Create) || event.Has(fsnotify.Write) {
						scheduleLoad(event.Name, func() {
							fileInfo,err := os.Stat(event.Name)
							if err!=nil{log.Println(event.Name, err); return}
							upsertNodes(map[string]int64{relPath: fileInfo.ModTime().Unix()})
							log.Println("A node has been updated: ",relPath)
						})
					}

				// If the file was in the templates folder of mandos.
				}else if mdTemplates[relPath] != nil{
					scheduleLoad(event.Name,func(){
						loadTemplate(relPath,"md")
						log.Println("A markdown template has been reloaded: ",relPath)
					})

				}else if partialTemplates[relPath] != nil{
					scheduleLoad(event.Name,func(){
						loadTemplate(relPath,"partial")
						log.Println("A partial template has been reloaded: ",relPath)
					})
				// Reload solo templates if the changed file was a solo template.
				}else if soloTemplates[relPath] != nil{
					scheduleLoad(event.Name,func(){
						loadTemplate(relPath,"solo")
						log.Println("A solo template has been reloaded: ",relPath)
					})
				}

				// If a new directory is created, watch it
				if event.Op&fsnotify.Create != 0 {
					info, err := os.Stat(event.Name)
					// Do not watch the static folder, as its always public.
					if err == nil && info.IsDir() && !strings.HasPrefix(event.Name, filepath.Join(notesPath,"static")){
						err = addWatchRecursive(filepath.Join(notesPath,relPath))
						if err != nil {log.Printf("Error adding new directory to watcher: %v", err)}
					}
				}
			}
		case err, ok := <-watcher.Errors: if !ok {return}; log.Println("Watcher error:", err)
		}
	}
}
