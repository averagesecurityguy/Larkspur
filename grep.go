package larkspur

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// defaultGrepMaxResults caps how many matches grepFiles collects when the
// caller doesn't specify max_results, so an overly broad pattern over a
// large tree can't produce an unbounded response.
const defaultGrepMaxResults = 200

// binarySniffBytes is how much of a file grepFiles reads to decide whether
// it looks like binary data (and should be skipped) before searching its
// content line by line.
const binarySniffBytes = 512

type grepFilesArgs struct {
	Pattern    string `json:"pattern"`
	Path       string `json:"path"`
	Glob       string `json:"glob"`
	MaxResults int    `json:"max_results"`
}

// looksBinary reports whether data appears to be non-text content, using
// the presence of a NUL byte as a cheap, standard heuristic (the same one
// git and grep itself use) — good enough to skip compiled binaries and
// other non-source files without needing a full content-type check.
func looksBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) != -1
}

// grepMatches searches a single file for pattern, appending "path:line:
// text" for each matching line to matches, until matches (shared across
// every file grepFiles walks) reaches max in total. It skips files that
// look binary or that it can't open, rather than failing the whole search
// over one unreadable or non-text file.
func grepMatches(path string, pattern *regexp.Regexp, max int, matches *[]string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	sniff := make([]byte, binarySniffBytes)
	n, _ := f.Read(sniff)
	if looksBinary(sniff[:n]) {
		return
	}

	if _, err := f.Seek(0, 0); err != nil {
		return
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++

		line := scanner.Text()
		if !pattern.MatchString(line) {
			continue
		}

		*matches = append(*matches, fmt.Sprintf("%s:%d: %s", path, lineNum, line))
		if len(*matches) >= max {
			return
		}
	}
}

// grepFiles searches files under path (recursively) for lines matching
// pattern, a regular expression (Go's RE2 syntax, the same as any-llm's
// providers expose no shell so there's no need to worry about shell
// quoting here). glob, if given, restricts the search to files whose base
// name matches it (e.g. "*.go"). .git directories are always skipped,
// since they hold repository internals rather than source content.
func grepFiles(arguments string) string {
	var args grepFilesArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("grep_files: error: %v", err)
	}

	if args.Pattern == "" {
		log.Error().Msg("missing pattern")
		return fmt.Sprintf("grep_files: error: missing pattern")
	}

	path := args.Path
	if path == "" {
		path = "."
	}

	maxResults := args.MaxResults
	if maxResults <= 0 {
		maxResults = defaultGrepMaxResults
	}

	pattern, err := regexp.Compile(args.Pattern)
	if err != nil {
		log.Error().Err(err).Msg("invalid pattern")
		return fmt.Sprintf("grep_files: error: invalid pattern: %v", err)
	}

	var matches []string

	err = filepath.WalkDir(filepath.Clean(path), func(entryPath string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}

		if d.IsDir() {
			if d.Name() == ".git" {
				return fs.SkipDir
			}
			return nil
		}

		if len(matches) >= maxResults {
			return fs.SkipAll
		}

		if args.Glob != "" {
			ok, err := filepath.Match(args.Glob, d.Name())
			if err != nil || !ok {
				return nil
			}
		}

		grepMatches(entryPath, pattern, maxResults, &matches)

		return nil
	})
	if err != nil {
		log.Error().Err(err).Msg("could not walk path")
		return fmt.Sprintf("grep_files: error: could not search %s", path)
	}

	if len(matches) == 0 {
		return "No matches found"
	}

	result := strings.Join(matches, "\n")

	if len(matches) >= maxResults {
		result += fmt.Sprintf("\n...stopped at %d matches; narrow pattern, glob, or path to see more.", maxResults)
	}

	return result
}
