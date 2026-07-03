package larkspur

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

type toolList struct {
	Tools []anyllm.Tool `json:"tools"`
}

// executeTool calls the appropriate function based on the tool name.
func executeTool(name, arguments string) string {
	switch name {
	case "system_command":
		return systemCommand(arguments)
	case "file_write_full":
		return fileWriteFull(arguments)
	case "file_read_full":
		return fileReadFull(arguments)
	case "file_read_lines":
		return fileReadLines(arguments)
	default:
		return fmt.Sprintf("error: unknown tool: %s", name)
	}
}

// loadSystemTools loads the list of system tools defined in the system.json
// file in the tools folder.
func loadTools(path string) []anyllm.Tool {
	var tl toolList

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Printf("Could not load tools: %v", err)
		return []anyllm.Tool{}
	}

	err = json.Unmarshal(data, &tl)
	if err != nil {
		fmt.Printf("Could not load tools: %v", err)
		return []anyllm.Tool{}
	}

	return tl.Tools
}

// LoadAllTools loads all of the tools from the various json files in the
// tools folder.
func LoadAllTools() []anyllm.Tool {
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
