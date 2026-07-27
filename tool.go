package larkspur

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

type toolList struct {
	Tools []anyllm.Tool `json:"tools"`
}

// maxToolOutputChars caps how much of a single tool result is fed back into
// the conversation. Derived from compactThreshold (chat.go) rather than its
// own bare number, so it stays proportional if that budget is ever retuned:
// an unbounded result (a large file, a verbose shell command) can otherwise
// burn most of a single try's share of the budget and starve every turn
// after it.
const maxToolOutputChars = compactThreshold / 10

// executeTool calls the appropriate function based on the tool name.
func executeTool(name, arguments string) string {
	var result string

	switch name {
	case "system_command":
		result = systemCommand(arguments)
	case "file_write_full":
		result = fileWriteFull(arguments)
	case "file_read_full":
		result = fileReadFull(arguments)
	case "file_read_lines":
		result = fileReadLines(arguments)
	case "file_size_bytes":
		result = fileSizeBytes(arguments)
	case "file_size_lines":
		result = fileSizeLines(arguments)
	case "file_find_glob":
		result = fileFindGlob(arguments)
	default:
		log.Error().Msgf("error: unknown tool: %s", name)
		return fmt.Sprintf("error: unknown tool: %s", name)
	}

	return truncateToolOutput(name, result)
}

// truncateToolOutput trims a tool result down to maxToolOutputChars so a
// single call cannot dominate the shared context window.
func truncateToolOutput(name, result string) string {
	if len(result) <= maxToolOutputChars {
		return result
	}

	log.Warn().
		Str("tool", name).
		Int("size", len(result)).
		Int("limit", maxToolOutputChars).
		Msg("tool output truncated")

	return fmt.Sprintf(
		"%s\n...output truncated at %d of %d characters. Use a more targeted tool call (e.g. file_read_lines) to see more.",
		result[:maxToolOutputChars], maxToolOutputChars, len(result),
	)
}

// loadTools loads the list of tools defined in the given path
// file in the tools folder.
func loadTools(path string) []anyllm.Tool {
	var tl toolList

	data, err := os.ReadFile(path)
	if err != nil {
		log.Error().Err(err).Msg("could not load tools")
		return []anyllm.Tool{}
	}

	err = json.Unmarshal(data, &tl)
	if err != nil {
		log.Error().Err(err).Msg("could not load tools")
		return []anyllm.Tool{}
	}

	return tl.Tools
}

// loadAllTools loads all of the tools from the various json files in the
// tools folder.
func loadAllTools() []anyllm.Tool {
	var all []anyllm.Tool

	// Load the system tools
	for _, tool := range loadTools(filepath.Join(".", "tools", "system.json")) {
		all = append(all, tool)
	}

	// Load the file tools
	for _, tool := range loadTools(filepath.Join(".", "tools", "file.json")) {
		all = append(all, tool)
	}
	return all
}
