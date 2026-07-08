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

// fileWriteFull writes the given content to the given filename.
func fileWriteFull(arguments string) string {
	var args fileWriteFullArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse JSON")
		return fmt.Sprintf("file_write_full: error: could not parse JSON")
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
		return fmt.Sprintf("file_read_full: error: could not parse JSON")
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
		return fmt.Sprintf("file_read_lines: error: could not parse arguments")
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
		return fmt.Sprintf("file_size_line: error: could not parse arguments")
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
		if err != nil && err != io.EOF {
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
		return fmt.Sprintf("file_size_bytes: error: could not parse arguments")
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
		return fmt.Sprintf("file_find_glob: error: could not parse arguments")
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
