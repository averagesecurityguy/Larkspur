package larkspur

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

type toolList struct {
	Tools []anyllm.Tool `json:"tools"`
}

// maxToolOutputChars caps how much of a single tool result is fed back into
// the conversation. Derived from a's own compactThreshold (chat.go) rather
// than its own bare number, so it stays proportional if that budget is ever
// retuned: an unbounded result (a large file, a verbose shell command) can
// otherwise burn most of a single try's share of the budget and starve
// every turn after it.
func (a *agent) maxToolOutputChars() int {
	return a.compactThreshold() / 10
}

// executeTool calls the appropriate function based on the tool name. a is
// the agent whose turn is currently running, used only to size the result
// truncation below to its own budget.
func executeTool(a *agent, name, arguments string) string {
	var result string

	switch name {
	case "system_command":
		result = systemCommand(arguments)
	case "dir_current":
		result = dirCurrent(arguments)
	case "find_executable":
		result = findExecutable(arguments)
	case "run_starlark":
		result = runStarlark(arguments)
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
	case "file_edit":
		result = fileEdit(arguments)
	case "file_delete":
		result = fileDelete(arguments)
	case "file_move":
		result = fileMove(arguments)
	case "dir_create":
		result = dirCreate(arguments)
	case "dir_list":
		result = dirList(arguments)
	case "grep_files":
		result = grepFiles(arguments)
	case "memory_search":
		result = memorySearch(arguments)
	case "memory_get":
		result = memoryGet(arguments)
	case "memory_put":
		result = memoryPut(arguments)
	default:
		log.Error().Msgf("error: unknown tool: %s", name)
		return fmt.Sprintf("error: unknown tool: %s", name)
	}

	return truncateToolOutput(a, name, result)
}

// truncateToolOutput trims a tool result down to a's maxToolOutputChars so
// a single call cannot dominate a's share of the shared context window.
func truncateToolOutput(a *agent, name, result string) string {
	limit := a.maxToolOutputChars()
	if len(result) <= limit {
		return result
	}

	log.Warn().
		Str("tool", name).
		Int("size", len(result)).
		Int("limit", limit).
		Msg("tool output truncated")

	return fmt.Sprintf(
		"%s\n...output truncated at %d of %d characters. Use a more targeted tool call (e.g. file_read_lines) to see more.",
		result[:limit], limit, len(result),
	)
}

// loadTools loads the list of tools defined in the given path
// file in the tools folder.
func loadTools(path string) []anyllm.Tool {
	var tl toolList

	data, err := assets.ReadFile(filepath.Clean(path))
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
	for _, tool := range loadTools(filepath.Join("tools", "system.json")) {
		all = append(all, tool)
	}

	// Load the file tools
	for _, tool := range loadTools(filepath.Join("tools", "file.json")) {
		all = append(all, tool)
	}

	// Load the memory tools
	for _, tool := range loadTools(filepath.Join("tools", "memory.json")) {
		all = append(all, tool)
	}

	// Load the scripting tools
	for _, tool := range loadTools(filepath.Join("tools", "script.json")) {
		all = append(all, tool)
	}

	return all
}

// loadExecutionTools loads every tool available to agents that carry out and
// verify plan objectives (developer, generalist), including task_checkpoint.
// Plan creation and verification use loadAllTools instead, since
// task_checkpoint only makes sense once a plan, and therefore a planID,
// exists.
func loadExecutionTools() []anyllm.Tool {
	all := loadAllTools()

	for _, tool := range loadTools(filepath.Join("tools", "checkpoint.json")) {
		all = append(all, tool)
	}

	return all
}
