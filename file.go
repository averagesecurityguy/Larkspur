package larkspur

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// fileWriteFull writes the given content to the given filename.
func fileWriteFull(arguments string) (string, error) {
	var args struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		return "", fmt.Errorf("file_write_full: error: %v", err)
	}

	err = os.WriteFile(filepath.Clean(args.Name), []byte(args.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("file_write_full: error: %v", err)
	}

	return "file_write_full: success", nil
}
