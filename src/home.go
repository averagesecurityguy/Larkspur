package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	// larkspurDirName is the name of larkspur's per-user data directory,
	// created under the user's home directory on startup.
	larkspurDirName = ".larkspur"

	// memoriesDBName is the name of the bbolt database file, within the
	// larkspur data directory, that backs the memory store.
	memoriesDBName = "memories.db"
)

// larkspurHomeDir returns the path to larkspur's per-user data directory,
// creating it if it does not already exist. The location is hard-coded to
// ~/.larkspur rather than configurable, so every invocation of larkspur
// shares the same memories regardless of where it is run from.
func larkspurHomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Could not determine home directory: %v\n", err)
		os.Exit(exitCodeNoHomeDir)
	}

	dir := filepath.Join(home, larkspurDirName)

	err = os.MkdirAll(dir, 0700)
	if err != nil {
		fmt.Printf("Could not create %s: %v\n", dir, err)
		os.Exit(exitCodeNoHomeDir)
	}

	return dir
}

// memoriesDBPath returns the hard-coded path to larkspur's memories
// database file within its home directory.
func memoriesDBPath() string {
	return filepath.Join(larkspurHomeDir(), memoriesDBName)
}
