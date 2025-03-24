package main

import (
	"os"
	"log"
	"path/filepath"
	"fmt"
	"github.com/fsnotify/fsnotify"
)

//var watchlock bool

func watchFileChanges() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {log.Fatal(err)}
	defer watcher.Close()

	// Helper function to add a directory and all its subdirectories to the watcher
	addWatchRecursive := func(root string) error {
		// Walk the directory tree
		return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil {return err}
			if info.IsDir() {
				err := watcher.Add(path)
				if err != nil {
					return fmt.Errorf("error adding directory %s to watcher: %w", path, err)
				}
			}
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
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) != 0 {
				loadTemplates()
				loadNotesAndAttachments()

				// If a new directory is created, watch it
				if event.Op&fsnotify.Create != 0 {
					info, err := os.Stat(event.Name)
					if err == nil && info.IsDir() {
						err = addWatchRecursive(event.Name)
						if err != nil {log.Printf("Error adding new directory to watcher: %v", err)}
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {return}
			log.Println("Watcher error:", err)
		}
	}
}
