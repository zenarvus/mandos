package main
import ("fmt";"log";"os";"path/filepath";"strings";"time"; "github.com/fsnotify/fsnotify")
func watchFileChanges() {
	var tReloadDebounceTimer *time.Timer // For template re-loading
	tDebounceMu := make(chan struct{}, 1) // simple mutex to avoid races
	var nReloadDebounceTimer *time.Timer // For node and attachment reloading.
	nDebounceMu := make(chan struct{}, 1)
	// This code implements a debounce: it schedules a function to run 5 seconds after the last call to scheduleLoad. Repeated calls within the 5s window reset the timer so only the final call's handler runs.
	scheduleLoad := func(run func(), debounceTimer **time.Timer, debounceMu chan struct{}) {
		// ensure only one goroutine manipulates timer at a time
		//fmt.Println("A function execution request has been made.")
		debounceMu <- struct{}{}
		if *debounceTimer != nil {(*debounceTimer).Stop()}
		*debounceTimer = time.AfterFunc(5 * time.Second, func() {
			//fmt.Println("Only this will be handled")
			run()
		})
		<-debounceMu
	}
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
				// If the file was in the templates folder of mandos.
				if strings.HasPrefix(event.Name, getEnvValue("MD_TEMPLATES")){
					scheduleLoad(func(){loadTemplates("md")}, &tReloadDebounceTimer, tDebounceMu)
				// If the file was not in the templates folder.
				}else{scheduleLoad(func(){
					// Reload solo templates if the changed file was a solo template.
					if soloTemplates[strings.TrimSuffix(strings.TrimPrefix(event.Name,notesPath),"~")] != nil{
						loadTemplates("solo");
					}else{loadNotesAndAttachments();}
				}, &nReloadDebounceTimer, nDebounceMu)}
				// If a new directory is created, watch it
				if event.Op&fsnotify.Create != 0 {
					info, err := os.Stat(event.Name)
					// Do not watch the static folder, as its always public.
					if err == nil && info.IsDir() && !strings.HasPrefix(event.Name, filepath.Join(notesPath,"static")){
						err = addWatchRecursive(filepath.Join(notesPath,event.Name))
						if err != nil {log.Printf("Error adding new directory to watcher: %v", err)}
					}
				}
			}
		case err, ok := <-watcher.Errors: if !ok {return}; log.Println("Watcher error:", err)
		}
	}
}
