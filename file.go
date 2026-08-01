package larkspur

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

type fileReadFullArgs struct {
	Name string `json:"file_name"`
}

type fileFindGlobArgs struct {
	Glob string `json:"glob"`
}

type fileWriteFullArgs struct {
	fileReadFullArgs
	Content string `json:"content"`
}

type fileReadLinesArgs struct {
	fileReadFullArgs
	Start int `json:"start_line"`
	Stop  int `json:"stop_line"`
}

type fileEditArgs struct {
	fileReadFullArgs
	OldString  string `json:"old_string"`
	NewString  string `json:"new_string"`
	ReplaceAll bool   `json:"replace_all"`
}

type dirCreateArgs struct {
	Path string `json:"path"`
}

type fileMoveArgs struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type dirListArgs struct {
	Path string `json:"path"`
}

// fileWriteFull writes the given content to the given filename.
func fileWriteFull(arguments string) string {
	var args fileWriteFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse JSON")
		return fmt.Sprintf("file_write_full: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Err(err).Msg("missing file name")
		return fmt.Sprintf("file_write_full: error: missing file name")
	}

	err = os.WriteFile(filepath.Clean(args.Name), []byte(args.Content), 0644)
	if err != nil {
		log.Error().Err(err).Msg("could not write file")
		return fmt.Sprintf("file_write_full: error: could not write file")
	}

	return "file_write_full: success"
}

// fileReadFull reads the content of the given filename.
func fileReadFull(arguments string) string {
	var args fileReadFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse JSON")
		return fmt.Sprintf("file_read_full: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Err(err).Msg("missing file name")
		return fmt.Sprintf("file_read_full: error: missing file name")
	}

	data, err := os.ReadFile(filepath.Clean(args.Name))
	if err != nil {
		log.Error().Err(err).Msg("could not read file")
		return fmt.Sprintf("file_read_full: error: could not read file")
	}

	return string(data)
}

// fileReadLines reads the content of the given filename from start to end.
func fileReadLines(arguments string) string {
	var args fileReadLinesArgs
	var lines []string

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_read_lines: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Err(err).Msg("missing file name")
		return fmt.Sprintf("file_read_lines: error: missing file name")
	}

	f, err := os.Open(filepath.Clean(args.Name))
	if err != nil {
		log.Error().Err(err).Msg("could not open file")
		return fmt.Sprintf("file_read_lines: error: could not open file")
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
			log.Error().Err(err).Msg("could not read file")
			return fmt.Sprintf("file_read_lines: error: could not read file")
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

// fileSizeLine returns a count of the number of lines in a file.
func fileSizeLines(arguments string) string {
	var args fileReadFullArgs
	var count int
	var read int
	var target []byte = []byte("\n")

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_size_line: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Err(err).Msg("missing file name")
		return fmt.Sprintf("file_size_line: error: missing file name")
	}

	f, err := os.Open(filepath.Clean(args.Name))
	if err != nil {
		log.Error().Err(err).Msg("could not open file")
		return fmt.Sprintf("file_size_line: error: could not open file")
	}

	buffer := make([]byte, 32*1024)

	for {
		read, err = f.Read(buffer)
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Error().Err(err).Msg("could not read file")
			return fmt.Sprintf("file_size_line: error: could not read file")
		}

		count += bytes.Count(buffer[:read], target)
	}

	return fmt.Sprintf("Line count: %d", count)
}

// fileSizeBytes returns the number of bytes in a file.
func fileSizeBytes(arguments string) string {
	var args fileReadFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_size_bytes: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Err(err).Msg("missing file name")
		return fmt.Sprintf("file_size_bytes: error: missing file name")
	}

	f, err := os.Open(filepath.Clean(args.Name))
	if err != nil {
		log.Error().Err(err).Msg("could not open file")
		return fmt.Sprintf("file_size_bytes: error: could not open file")
	}

	stat, err := f.Stat()
	if err != nil {
		log.Error().Err(err).Msg("could not get file stats")
		return fmt.Sprintf("file_size_bytes: error: could not get file stats")
	}

	return fmt.Sprintf("Size in bytes: %d", stat.Size())
}

func fileFindGlob(arguments string) string {
	var args fileFindGlobArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_find_glob: error: %v", err)
	}

	if args.Glob == "" {
		log.Error().Err(err).Msg("missing glob pattern")
		return fmt.Sprintf("file_find_glob: error: missing glob pattern")
	}

	matches, err := filepath.Glob(args.Glob)
	if err != nil {
		log.Error().Err(err).Msg("bad glob pattern")
		return fmt.Sprintf("file_find_glob: error: bad glob pattern")
	}

	if matches == nil {
		log.Info().Msg("no matching files")
		return fmt.Sprintf("No matching files")
	}

	return strings.Join(matches, "\n")
}

// fileEdit replaces old_string with new_string in the given file. Unless
// replace_all is set, old_string must match exactly once in the file — this
// forces the caller to include enough surrounding context to pin down a
// single location, the same way it would need to for the edit to be
// unambiguous to a human reader, rather than silently guessing which
// occurrence was meant.
func fileEdit(arguments string) string {
	var args fileEditArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_edit: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Msg("missing file name")
		return fmt.Sprintf("file_edit: error: missing file name")
	}

	if args.OldString == "" {
		log.Error().Msg("missing old_string")
		return fmt.Sprintf("file_edit: error: missing old_string")
	}

	if args.OldString == args.NewString {
		log.Error().Msg("old_string and new_string are identical")
		return fmt.Sprintf("file_edit: error: old_string and new_string are identical")
	}

	path := filepath.Clean(args.Name)

	data, err := os.ReadFile(path)
	if err != nil {
		log.Error().Err(err).Msg("could not read file")
		return fmt.Sprintf("file_edit: error: could not read file")
	}

	content := string(data)
	count := strings.Count(content, args.OldString)

	if count == 0 {
		log.Error().Msg("old_string not found")
		return fmt.Sprintf("file_edit: error: old_string not found in file")
	}

	if count > 1 && !args.ReplaceAll {
		return fmt.Sprintf(
			"file_edit: error: old_string appears %d times in the file; "+
				"set replace_all to true to replace every occurrence, "+
				"or include more surrounding context so old_string matches only one location",
			count,
		)
	}

	limit := 1
	if args.ReplaceAll {
		limit = -1
	}

	updated := strings.Replace(content, args.OldString, args.NewString, limit)

	err = os.WriteFile(path, []byte(updated), 0644)
	if err != nil {
		log.Error().Err(err).Msg("could not write file")
		return fmt.Sprintf("file_edit: error: could not write file")
	}

	if args.ReplaceAll {
		return fmt.Sprintf("file_edit: success (replaced %d occurrences)", count)
	}

	return "file_edit: success"
}

// fileDelete removes the given file. It refuses to remove a directory —
// deleting a whole directory tree is a much larger, harder to undo action
// than deleting one file, and isn't something this tool takes on.
func fileDelete(arguments string) string {
	var args fileReadFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_delete: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Msg("missing file name")
		return fmt.Sprintf("file_delete: error: missing file name")
	}

	path := filepath.Clean(args.Name)

	info, err := os.Stat(path)
	if err != nil {
		log.Error().Err(err).Msg("could not stat file")
		return fmt.Sprintf("file_delete: error: could not find file")
	}

	if info.IsDir() {
		log.Error().Msg("refusing to delete a directory")
		return fmt.Sprintf("file_delete: error: %s is a directory, not a file", path)
	}

	if err := os.Remove(path); err != nil {
		log.Error().Err(err).Msg("could not delete file")
		return fmt.Sprintf("file_delete: error: could not delete file")
	}

	return "file_delete: success"
}

// dirCreate creates the given directory, along with any missing parent
// directories, the same way "mkdir -p" does. It is a no-op, not an error,
// if the directory already exists.
func dirCreate(arguments string) string {
	var args dirCreateArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("dir_create: error: %v", err)
	}

	if args.Path == "" {
		log.Error().Msg("missing path")
		return fmt.Sprintf("dir_create: error: missing path")
	}

	if err := os.MkdirAll(filepath.Clean(args.Path), 0755); err != nil {
		log.Error().Err(err).Msg("could not create directory")
		return fmt.Sprintf("dir_create: error: could not create directory")
	}

	return "dir_create: success"
}

// fileMove moves or renames source to destination. It works for both files
// and directories, but does not create destination's parent directory —
// use dir_create first if it doesn't exist yet.
func fileMove(arguments string) string {
	var args fileMoveArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("file_move: error: %v", err)
	}

	if args.Source == "" || args.Destination == "" {
		log.Error().Msg("missing source or destination")
		return fmt.Sprintf("file_move: error: missing source or destination")
	}

	src := filepath.Clean(args.Source)
	dst := filepath.Clean(args.Destination)

	if err := os.Rename(src, dst); err != nil {
		log.Error().Err(err).Msg("could not move file")
		return fmt.Sprintf("file_move: error: could not move %s to %s", src, dst)
	}

	return "file_move: success"
}

// dirList lists the immediate contents of a directory (not recursive; use
// file_find_glob for recursive matching), one entry per line as
// "type\tsize\tname". size is "-" for directories.
func dirList(arguments string) string {
	var args dirListArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("dir_list: error: %v", err)
	}

	path := args.Path
	if path == "" {
		path = "."
	}

	entries, err := os.ReadDir(filepath.Clean(path))
	if err != nil {
		log.Error().Err(err).Msg("could not read directory")
		return fmt.Sprintf("dir_list: error: could not read directory")
	}

	if len(entries) == 0 {
		return "<empty directory>"
	}

	var result strings.Builder

	for _, entry := range entries {
		if entry.IsDir() {
			fmt.Fprintf(&result, "dir\t-\t%s\n", entry.Name())
			continue
		}

		size := "?"
		if info, err := entry.Info(); err == nil {
			size = fmt.Sprintf("%d", info.Size())
		}

		fmt.Fprintf(&result, "file\t%s\t%s\n", size, entry.Name())
	}

	return strings.TrimSuffix(result.String(), "\n")
}
