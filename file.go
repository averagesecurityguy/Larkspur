package larkspur

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type fileReadFullArgs struct {
	Name string `json:"name"`
}

type fileWriteFullArgs struct {
	fileReadFullArgs
	Content string `json:"content"`
}

type fileReadLinesArgs struct {
	fileReadFullArgs
	Start int `json:"start"`
	Stop  int `json:"stop"`
}

// fileWriteFull writes the given content to the given filename.
func fileWriteFull(arguments string) string {
	var args fileWriteFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return fmt.Sprintf("file_write_full: error: %v", err)
	}

	if args.Name == "" {
		return fmt.Sprintf("file_write_full: error: missing file name")
	}

	err = os.WriteFile(filepath.Clean(args.Name), []byte(args.Content), 0644)
	if err != nil {
		return fmt.Sprintf("file_write_full: error: %v", err)
	}

	return "file_write_full: success"
}

// fileReadFull reads the content of the given filename.
func fileReadFull(arguments string) string {
	var args fileReadFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return fmt.Sprintf("file_read_full: error: %v", err)
	}

	if args.Name == "" {
		return fmt.Sprintf("file_read_full: error: missing file name")
	}

	data, err := os.ReadFile(filepath.Clean(args.Name))
	if err != nil {
		return fmt.Sprintf("file_read_full: error: %v", err)
	}

	return string(data)
}

// fileReadLines reads the content of the given filename from start to end.
func fileReadLines(arguments string) string {
	var args fileReadLinesArgs
	var lines []string

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return fmt.Sprintf("file_read_lines: error: %v", err)
	}

	if args.Name == "" {
		return fmt.Sprintf("file_read_lines: error: missing file name")
	}

	f, err := os.Open(filepath.Clean(args.Name))
	if err != nil {
		return fmt.Sprintf("file_read_lines: error: %v", err)
	}

	defer f.Close()

	scanner := bufio.NewReader(f)
	count := 0

	for {
		count = count + 1

		line, err := scanner.ReadString('\n')
		if err == io.EOF {
			if len(line) != 0 {
				lines = append(lines, strings.TrimSuffix(line, "\n"))
			}
			break
		}

		if err != nil {
			return fmt.Sprintf("file_read_lines: error: %v", err)
		}

		if count < args.Start {
			continue
		}

		if count > args.Stop {
			break
		}

		lines = append(lines, strings.TrimSuffix(line, "\n"))
	}

	return fmt.Sprintf("%s\n", strings.Join(lines, "\n"))
}
