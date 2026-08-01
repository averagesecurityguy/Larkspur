package larkspur

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type safeTest struct {
	argv []string
	safe bool
}

type systemCommandTest struct {
	args     string
	response string
}

func TestCommand(t *testing.T) {
	t.Run("Testing safeCommand", testSafeCommand)
	t.Run("Testing SystemCommand", testSystemCommand)
	t.Run("Testing SystemCommand shell metacharacters are inert", testSystemCommandNoShell)
	t.Run("Testing SystemCommand chained commands", testSystemCommandChaining)
	t.Run("Testing dirCurrent", testDirCurrent)
	t.Run("Testing findExecutable", testFindExecutable)
}

func testSafeCommand(t *testing.T) {
	fmt.Println(t.Name())

	safeTests := []safeTest{
		// Exact-match commands.
		{argv: []string{"ps"}, safe: true},
		{argv: []string{"gofmt"}, safe: true},
		{argv: []string{"mypy"}, safe: true},

		// Removed on purpose: covered by dedicated built-in tools instead
		// (dir_current, find_executable, and the file/dir/grep tools).
		{argv: []string{"pwd"}, safe: false},
		{argv: []string{"which"}, safe: false},
		{argv: []string{"cat"}, safe: false},
		{argv: []string{"ls"}, safe: false},
		{argv: []string{"grep"}, safe: false},
		{argv: []string{"find"}, safe: false},
		{argv: []string{"head"}, safe: false},
		{argv: []string{"tail"}, safe: false},

		// Never allowed at all.
		{argv: []string{"git"}, safe: false},
		{argv: []string{"python3"}, safe: false},
		{argv: []string{"npm"}, safe: false},
		{argv: []string{"make"}, safe: false},

		// go/cargo: only specific subcommands are allowed, and only with
		// a subcommand present at all.
		{argv: []string{"go"}, safe: false},
		{argv: []string{"go", "build"}, safe: true},
		{argv: []string{"go", "vet"}, safe: true},
		{argv: []string{"go", "fmt"}, safe: true},
		{argv: []string{"go", "run"}, safe: false},
		{argv: []string{"go", "get"}, safe: false},
		{argv: []string{"cargo"}, safe: false},
		{argv: []string{"cargo", "clippy"}, safe: true},
		{argv: []string{"cargo", "check"}, safe: true},
		{argv: []string{"cargo", "install"}, safe: false},
		{argv: []string{"cargo", "run"}, safe: false},
	}

	for _, st := range safeTests {
		resp := safeCommand(st.argv)
		if resp != st.safe {
			t.Fatalf("Expected `%t`, received `%t` for argv `%v`", st.safe, resp, st.argv)
		}
	}
}

func testSystemCommand(t *testing.T) {
	fmt.Println(t.Name())

	cmdTests := []systemCommandTest{
		{args: `{"unexpected": "not a valid key"}`, response: "system_command: error: missing command"},
		{args: `{command": "gofmt junk"}`, response: "system_command: error"},
		{args: `{"command": "git"}`, response: "system_command: error: command not allowed"},
		{args: `{"command": "go run main.go"}`, response: "system_command: error: command not allowed"},
		{args: `{"command": "pwd"}`, response: "system_command: error: command not allowed"},
		{args: `{"command": "gofmt /tmp/does_not_exist_larkspur_test.go"}`, response: "no such file or directory"},
		{args: `{"command": "python3 --version"}`, response: "system_command: error: command not allowed"},
	}

	for _, ct := range cmdTests {
		resp := systemCommand(ct.args)

		if !strings.Contains(resp, ct.response) {
			t.Fatalf("Expected response to contain `%s`, received `%s`", ct.response, resp)
		}
	}
}

// testSystemCommandNoShell proves that shell metacharacters in the
// "command" argument have no special effect: since systemCommand execs the
// allow-listed command directly rather than via a shell, chaining,
// redirection, and command substitution all become inert literal argument
// text instead of being interpreted. gofmt is used here since this project
// already depends on the Go toolchain being present to run its own tests.
func testSystemCommandNoShell(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()
	one := filepath.Join(dir, "one.go")

	if err := os.WriteFile(one, []byte("package main\n"), 0644); err != nil {
		t.Fatalf("could not WriteFile %s: %v", one, err)
	}

	// A trailing "; git status" is never interpreted as a second command:
	// argv[0] is "gofmt" (allowed), and everything else — including
	// "git" — is just literal argument text handed to gofmt, which fails
	// to find files by those literal names rather than running git.
	resp := systemCommand(fmt.Sprintf(`{"command": "gofmt %s/nope.go; git status"}`, dir))
	if !strings.Contains(resp, "no such file or directory") {
		t.Fatalf("Expected a file-not-found error, received `%s`", resp)
	}
	if strings.Contains(resp, "On branch") || strings.Contains(resp, "not a git repository") {
		t.Fatalf("git appears to have actually run, received `%s`", resp)
	}

	// A redirection operator is likewise just a literal argument: gofmt
	// receives ">" and the target path as plain arguments to format, and
	// no file is created at that path.
	target := filepath.Join(dir, "pwned")
	resp = systemCommand(fmt.Sprintf(`{"command": "gofmt %s > %s"}`, one, target))
	if !strings.Contains(resp, "package main") {
		t.Fatalf("Expected gofmt to still format one.go, received `%s`", resp)
	}
	if _, err := os.Stat(target); err == nil {
		t.Fatalf("Expected no file to be created at %s via a fake redirect", target)
	}
}

// testSystemCommandChaining exercises && as the one supported chaining
// operator: every stage must be independently allow-listed before any of
// them run, and a failing stage stops the chain before the next one runs.
func testSystemCommandChaining(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()

	one := filepath.Join(dir, "one.go")
	two := filepath.Join(dir, "two.go")

	if err := os.WriteFile(one, []byte("package main\n\n// MARKERONE\n"), 0644); err != nil {
		t.Fatalf("could not WriteFile %s: %v", one, err)
	}
	if err := os.WriteFile(two, []byte("package main\n\n// MARKERTWO\n"), 0644); err != nil {
		t.Fatalf("could not WriteFile %s: %v", two, err)
	}

	// Two allowed stages both succeed: both outputs appear, in order.
	resp := systemCommand(fmt.Sprintf(`{"command": "gofmt %s && gofmt %s"}`, one, two))
	if !strings.Contains(resp, "MARKERONE") || !strings.Contains(resp, "MARKERTWO") {
		t.Fatalf("Expected both stages' output, received `%s`", resp)
	}
	if strings.Index(resp, "MARKERONE") > strings.Index(resp, "MARKERTWO") {
		t.Fatalf("Expected MARKERONE before MARKERTWO, received `%s`", resp)
	}

	// The first stage fails (missing file), so the second stage must
	// never run: its content must not appear in the response.
	resp = systemCommand(fmt.Sprintf(`{"command": "gofmt %s/nope.go && gofmt %s"}`, dir, two))
	if strings.Contains(resp, "MARKERTWO") {
		t.Fatalf("Expected the chain to stop before the second stage, received `%s`", resp)
	}
	if !strings.Contains(resp, "no such file or directory") {
		t.Fatalf("Expected a file-not-found error, received `%s`", resp)
	}

	// A disallowed command anywhere in the chain rejects the whole call
	// before any stage runs — not just the disallowed one.
	resp = systemCommand(fmt.Sprintf(`{"command": "gofmt %s && git status"}`, one))
	if resp != "system_command: error: command not allowed" {
		t.Fatalf("Expected the whole chain to be rejected, received `%s`", resp)
	}
}

func testDirCurrent(t *testing.T) {
	fmt.Println(t.Name())

	want, err := os.Getwd()
	if err != nil {
		t.Fatalf("could not Getwd: %v", err)
	}

	got := dirCurrent(`{}`)
	if got != want {
		t.Fatalf("Expected `%s`, received `%s`", want, got)
	}
}

func testFindExecutable(t *testing.T) {
	fmt.Println(t.Name())

	response := findExecutable(`{"unexpected": "not a valid key"}`)
	if !strings.Contains(response, "find_executable: error: missing name") {
		t.Fatalf("Expected a missing-name error, received `%s`", response)
	}

	response = findExecutable(`{"name": "nonexistent_cmd_xyz"}`)
	if !strings.Contains(response, "find_executable: not found") {
		t.Fatalf("Expected a not-found response, received `%s`", response)
	}

	response = findExecutable(`{"name": "gofmt"}`)
	if !strings.HasSuffix(response, string(filepath.Separator)+"gofmt") {
		t.Fatalf("Expected a path ending in gofmt, received `%s`", response)
	}
}
