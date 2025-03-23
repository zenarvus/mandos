package main

import (
	"os"
	"path/filepath"
	"io/fs"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

var watcher *fsnotify.Watcher

func addWatchers(dir string) error {
	err := watcher.Add(dir)
	if err != nil {return err}
	// Add watchers for subdirectories
	return filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {return err}
		if d.IsDir() && path != dir {
			err = watcher.Add(path)
			if err != nil {return err}
		}
		return nil
	})
}

func loadAndWatchNodesAndAttachments(){
	var err error
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("Error creating file watcher:", err)
		return
	}
	defer watcher.Close()

	// Initial load of notes and attachments
	loadNotesAndAttachments()

	// Watch for changes in the directory and subdirectories
	err = addWatchers(notesPath)
	if err != nil {
		fmt.Println("Error adding watchers:", err)
		return
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {return}

				if event.Op&fsnotify.Create == fsnotify.Create {
					// If a new directory is created, add it to the watcher
					fi, err := os.Stat(event.Name)
					if err == nil && fi.IsDir() {addWatchers(event.Name)}
				}
				// Reload notes and attachments when a file is added or modified
				loadNotesAndAttachments()
			case err, ok := <-watcher.Errors:
				if !ok {return}
				fmt.Println("Watcher error:", err)
			}
		}
	}()
}
