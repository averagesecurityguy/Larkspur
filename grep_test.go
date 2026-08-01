package larkspur

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGrep(t *testing.T) {
	t.Run("Testing grepFiles", testGrepFiles)
}

func testGrepFiles(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatalf("could not MkdirAll: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0755); err != nil {
		t.Fatalf("could not MkdirAll: %v", err)
	}

	files := map[string]string{
		"main.go":       "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n",
		"sub/helper.go": "package sub\n\nfunc Helper() int {\n\treturn 42\n}\n",
		"notes.txt":     "hello from a text file\n",
		".git/config":   "hello inside git internals, should be skipped\n",
	}

	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatalf("could not MkdirAll: %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("could not WriteFile %s: %v", path, err)
		}
	}

	// Missing pattern.
	response := grepFiles(`{"unexpected": "not a valid key"}`)
	if !strings.Contains(response, "grep_files: error: missing pattern") {
		t.Fatalf("Expected a missing-pattern error, received `%s`", response)
	}

	// Invalid regular expression.
	response = grepFiles(fmt.Sprintf(`{"pattern": "(", "path": %q}`, dir))
	if !strings.Contains(response, "grep_files: error: invalid pattern") {
		t.Fatalf("Expected an invalid-pattern error, received `%s`", response)
	}

	// Matches across file types, but never inside .git.
	response = grepFiles(fmt.Sprintf(`{"pattern": "hello", "path": %q}`, dir))
	if !strings.Contains(response, "main.go:4:") {
		t.Fatalf("Expected a match in main.go, received `%s`", response)
	}
	if !strings.Contains(response, "notes.txt:1:") {
		t.Fatalf("Expected a match in notes.txt, received `%s`", response)
	}
	if strings.Contains(response, ".git") {
		t.Fatalf("Expected .git to be skipped, received `%s`", response)
	}

	// glob restricts the search to matching file names.
	response = grepFiles(fmt.Sprintf(`{"pattern": "hello", "path": %q, "glob": "*.go"}`, dir))
	if !strings.Contains(response, "main.go:4:") {
		t.Fatalf("Expected a match in main.go, received `%s`", response)
	}
	if strings.Contains(response, "notes.txt") {
		t.Fatalf("Expected notes.txt to be excluded by the glob, received `%s`", response)
	}

	// A pattern that matches nothing.
	response = grepFiles(fmt.Sprintf(`{"pattern": "nothing_matches_this_xyz", "path": %q}`, dir))
	if response != "No matches found" {
		t.Fatalf("Expected `No matches found`, received `%s`", response)
	}

	// max_results caps the number of matches and notes that it did.
	response = grepFiles(fmt.Sprintf(`{"pattern": "hello", "path": %q, "max_results": 1}`, dir))
	if strings.Count(response, "\n") != 1 {
		t.Fatalf("Expected exactly one match line plus a truncation note, received `%s`", response)
	}
	if !strings.Contains(response, "stopped at 1 matches") {
		t.Fatalf("Expected a truncation note, received `%s`", response)
	}
}
