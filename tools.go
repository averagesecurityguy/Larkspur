package larkspur

import (
	"fmt"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// Define the models tools
var tools = []anyllm.Tool{
	{
		Type: "function",
		Function: anyllm.Function{
			Name:        "system_command",
			Description: "Execute a shell command.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"command": map[string]any{
						"type":        "string",
						"description": "The shell command to execute.",
					},
				},
			},
		},
	},
	{
		Type: "function",
		Function: anyllm.Function{
			Name:        "file_write_full",
			Description: "Write the contents to a file",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name": map[string]any{
						"type":        "string",
						"description": "The name of the file to write.",
					},
					"content": map[string]any{
						"type":        "string",
						"description": "The content to write to the file.",
					},
				},
			},
		},
	},
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
