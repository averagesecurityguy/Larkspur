package larkspur

import (
	"fmt"
	"path/filepath"
	"os"
	"encoding/json"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

type toolList struct {
	Tools []anyllm.Tool `json:"tools"`
}

// executeTool calls the appropriate function based on the tool name.
func executeTool(name, arguments string) (string, error) {
	var result string
	var err error

	switch name {
	case "system_command":
		result, err = systemCommand(arguments)
	case "file_write_full":
		result, err = fileWriteFull(arguments)
	default:
		return "", fmt.Errorf("Error: unknown tool: %s", name)
	}

	if err != nil {
		return "", fmt.Errorf("Error: %v", err)
	}

	return result, nil
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
