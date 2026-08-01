package larkspur

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/google/shlex"
)

// allowed lists commands system_command may run with any arguments —
// single-purpose tools with no unsafe mode of operation, so there's
// nothing further to restrict once the command name itself is allowed.
// Reading, writing, listing, and glob-finding files are handled by
// dedicated built-in tools now (file_read_full, file_read_lines,
// file_write_full, dir_list, file_find_glob, grep_files), so cat, head,
// tail, ls, find, and grep are deliberately not here anymore — anything
// they'd be used for has a built-in that doesn't depend on this allow list
// staying in sync with every way a model might phrase a shell command.
//
// Deliberately NOT included, even though they're common: npm, npx,
// python, python3, make, cmake, mvn, gradle. Each is a general-purpose
// task runner where safe and dangerous operations share the same argument
// grammar with no clean boundary to restrict — npm run/npx can execute any
// script or package, python -c/-m can run arbitrary code, make and cmake
// --build invoke arbitrary Makefile/build-system targets, and mvn/gradle
// can invoke arbitrary plugin goals (including ones that publish or
// deploy). go and cargo have the same shape (e.g. "go run", "cargo
// install"), but their few dangerous verbs are cleanly separable from
// their safe ones, so they're allowed below with only specific
// subcommands permitted (see allowedSubcommands) instead of being
// excluded entirely.
var allowed = []string{
	// General inspection, unrelated to any language toolchain. pwd and
	// which are deliberately not here — dir_current and find_executable
	// (below) cover them as built-ins, using os.Getwd and exec.LookPath
	// directly instead of shelling out.
	"ps",

	// Go: gofmt is a standalone binary, not a "go" subcommand.
	"gofmt", "golangci-lint",

	// JavaScript/TypeScript: standalone binaries, not run through npm/npx.
	"tsc", "eslint", "prettier",

	// Java: the compiler itself; there's no single standard linter to add
	// here, and maven/gradle are excluded for the reasons above.
	"javac",

	// C/C++: compilers plus dedicated static analyzers. make and cmake are
	// deliberately excluded (see above).
	"gcc", "clang", "cppcheck", "clang-tidy",

	// Python: dedicated lint/type-check tools only — never python/python3
	// itself (see above).
	"flake8", "pylint", "ruff", "mypy",
}

// allowedSubcommands lists the specific subcommands permitted for
// multi-purpose build tools that mix safe operations (build, vet, lint,
// format-check) with dangerous ones (running arbitrary code, fetching and
// executing dependencies, publishing packages) under the same command
// name. A command in this map is only allowed when argv[1] is one of its
// listed subcommands; anything else, including no subcommand at all, is
// rejected.
var allowedSubcommands = map[string][]string{
	// "go run" executes arbitrary code; "go get"/"go install"/"go mod"
	// fetch and can execute remote code. build/vet/fmt only compile,
	// statically analyze, or reformat.
	"go": {"build", "vet", "fmt"},

	// "cargo run"/"cargo install"/"cargo publish"/"cargo add" execute
	// arbitrary code, fetch dependencies, or publish packages. build/
	// check/clippy/fmt only compile, type-check, lint, or reformat.
	"cargo": {"build", "check", "clippy", "fmt"},
}

// commandTimeout bounds how long a whole system_command call may run —
// the entire && chain, not each stage individually — so an allowed-but-slow
// invocation, or a long chain of them, can't hang an agent's turn
// indefinitely.
const commandTimeout = 30 * time.Second

// safeCommand returns true if argv's command is exactly allowed, or is a
// multi-purpose build tool invoked with one of its explicitly permitted
// subcommands (allowedSubcommands).
func safeCommand(argv []string) bool {
	for _, cmd := range allowed {
		if argv[0] == cmd {
			return true
		}
	}

	subcommands, ok := allowedSubcommands[argv[0]]
	if !ok || len(argv) < 2 {
		return false
	}

	for _, sub := range subcommands {
		if argv[1] == sub {
			return true
		}
	}

	return false
}

// parseChain splits a command string on && into one argv per stage. &&
// is recognized only as a literal, space-delimited separator between
// otherwise-independent commands — never handed to a shell for real
// operator parsing — so it works as long as a stage's own arguments don't
// themselves contain the literal substring "&&". Every other shell
// metacharacter is left untouched inside each stage and is passed to that
// stage's own exec call as plain argument text.
func parseChain(command string) ([][]string, error) {
	stages := strings.Split(command, "&&")
	argvs := make([][]string, 0, len(stages))

	for _, stage := range stages {
		stage = strings.TrimSpace(stage)
		if stage == "" {
			return nil, fmt.Errorf("empty command between &&")
		}

		argv, err := shlex.Split(stage)
		if err != nil {
			return nil, fmt.Errorf("could not parse command %q: %w", stage, err)
		}

		if len(argv) == 0 {
			return nil, fmt.Errorf("empty command between &&")
		}

		argvs = append(argvs, argv)
	}

	return argvs, nil
}

// systemCommand executes a chain of one or more allow-listed commands
// joined by &&, and returns their combined stdout and stderr. Every stage
// must be independently allow-listed before anything runs — if any
// stage's command is not allowed, the whole call is rejected up front
// rather than executing some stages and not others. Stages then run in
// order, each directly via exec rather than through a shell, mirroring
// the shell's own && semantics: a stage only runs if the previous one
// exited successfully, and the chain stops at the first failure.
func systemCommand(arguments string) string {
	var args struct {
		Command string `json:"command"`
	}

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		return fmt.Sprintf("system_command: error: %v", err)
	}

	if args.Command == "" {
		return "system_command: error: missing command"
	}

	argvs, err := parseChain(args.Command)
	if err != nil {
		return fmt.Sprintf("system_command: error: %v", err)
	}

	for _, argv := range argvs {
		if !safeCommand(argv) {
			return "system_command: error: command not allowed"
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), commandTimeout)
	defer cancel()

	var output strings.Builder

	for i, argv := range argvs {
		cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
		out, runErr := cmd.CombinedOutput()

		output.Write(out)

		if runErr != nil {
			if i < len(argvs)-1 {
				fmt.Fprintf(&output, "\n(chain stopped after %q: %v)", strings.Join(argv, " "), runErr)
			}
			break
		}
	}

	return output.String()
}

// dirCurrent returns the process's current working directory, using
// os.Getwd directly rather than allowing "pwd" through system_command.
func dirCurrent(_ string) string {
	dir, err := os.Getwd()
	if err != nil {
		log.Error().Err(err).Msg("could not get working directory")
		return "dir_current: error: could not get working directory"
	}

	return dir
}

type findExecutableArgs struct {
	Name string `json:"name"`
}

// findExecutable looks up name on the process's PATH, using exec.LookPath
// directly rather than allowing "which" through system_command.
func findExecutable(arguments string) string {
	var args findExecutableArgs

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("find_executable: error: %v", err)
	}

	if args.Name == "" {
		log.Error().Msg("missing name")
		return "find_executable: error: missing name"
	}

	path, err := exec.LookPath(args.Name)
	if err != nil {
		return fmt.Sprintf("find_executable: not found: %s", args.Name)
	}

	return path
}
