package larkspur

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/google/shlex"
)

var (
	allowed = []string{"cat", "pwd", "ps", "head", "tail", "grep", "find", "ls", "which"}
)

// safeCommand returns true if the given command is in the allowed list.
func safeCommand(command string) bool {
	safe := false

	for _, cmd := range allowed {
		if command == cmd {
			safe = true
			break
		}
	}

	return safe
}

// safeSplit splits a shell string on the separator, lexes the split commands,
// and verifies they are all in the safe command list.
func safeSplit(shellStr, sep string) bool {
	safe := true
	commands := strings.Split(shellStr, sep)

	for _, command := range commands {
		if command == "" {
			continue
		}

		tokens, _ := shlex.Split(command)

		if safeCommand(tokens[0]) == false {
			safe = false
			break
		}
	}

	return safe
}

// safePipe returns true if all of the piped commands are safe commands.
func safePipe(shellStr string) bool {
	return safeSplit(shellStr, "|")
}

// safeEval returns true if the command inside an eval statement `$()` is
// safe.
func safeEval(shellStr string) bool {
	return safeSplit(shellStr, "$(")
}

// safeSemicolon returns true if all of the semicolon separated commands are
// safe.
func safeSemicolon(shellStr string) bool {
	return safeSplit(shellStr, ";")
}

// systemCommand executes a shell command and returns its combined stdout and
// stderr. Before executing a command, a few basic checks are run to ensure
// the commands being run are allowed. The checks are not comprehensive and
// will not stop a persistent attacker.
func systemCommand(arguments string) (string, error) {
	// Convert the JSON argument string to an args struct.
	var args struct {
		Command string `json:"command"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return "", fmt.Errorf("system_command: error: %v", err)
	}

	// Conduct a simple sanity check on our command
	shellStr := args.Command
	if strings.HasPrefix(shellStr, "$(") {
		shellStr = strings.TrimPrefix(shellStr, "$(")
		shellStr = strings.TrimSuffix(shellStr, ")")
	}

	if !safeEval(shellStr) || !safePipe(shellStr) {
		return "", fmt.Errorf("system_command: error: command not allowed")
	}

	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "sh", "-c", fmt.Sprintf("%s", shellStr))
	out, _ := cmd.CombinedOutput()

	return fmt.Sprintf("%s", out), nil
}
