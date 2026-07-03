package larkspur

import (
	"fmt"
	"strings"
	"testing"
)

type safeTest struct {
	command string
	safe    bool
}

type systemCommandTest struct {
	args     string
	err      string
	response string
}

func TestCommand(t *testing.T) {
	t.Run("Testing safeCommand", testSafeCommand)
	t.Run("Testing safePipe", testSafePipe)
	t.Run("Testing safeEval", testSafeEval)
	t.Run("Testing safeSemicolon", testSafeSemicolon)
	t.Run("Testing SystemCommand", testSystemCommand)
}

func testSafeCommand(t *testing.T) {
	fmt.Println(t.Name())

	safeTests := []safeTest{
		{command: "cat", safe: true},
		{command: "grep", safe: true},
		{command: "find", safe: true},
		{command: "ls", safe: true},
		{command: "git", safe: false},
	}

	for _, st := range safeTests {
		resp := safeCommand(st.command)
		if resp != st.safe {
			t.Fatalf("Expected `%t`, received `%t` for command `%s`", st.safe, resp, st.command)
		}
	}
}

func testSafePipe(t *testing.T) {
	fmt.Println(t.Name())

	safePipes := []safeTest{
		{command: "cat test | grep -irl", safe: true},
		{command: "grep | git", safe: false},
	}

	for _, st := range safePipes {
		resp := safePipe(st.command)
		if resp != st.safe {
			t.Fatalf("Expected `%t`, received `%t` for command `%s`", st.safe, resp, st.command)
		}
	}
}

func testSafeEval(t *testing.T) {
	fmt.Println(t.Name())

	safePipes := []safeTest{
		{command: "cat $(grep -irl) >> grep.text", safe: true},
		{command: "$(grep | git)", safe: true},
		{command: "$(git | grep)", safe: false},
		{command: "cat | grep | git", safe: true},
	}

	for _, st := range safePipes {
		resp := safeEval(st.command)
		if resp != st.safe {
			t.Fatalf("Expected `%t`, received `%t` for command `%s`", st.safe, resp, st.command)
		}
	}
}

func testSafeSemicolon(t *testing.T) {
	safeSemicolons := []safeTest{
		{command: "cat $(grep -irl) >> grep.text", safe: true},
		{command: "grep; git", safe: false},
		{command: "cat | grep; git", safe: false},
	}

	for _, st := range safeSemicolons {
		resp := safeSemicolon(st.command)
		if resp != st.safe {
			t.Fatalf("Expected `%t`, received `%t` for command `%s`", st.safe, resp, st.command)
		}
	}
}

func testSystemCommand(t *testing.T) {
	cmdTests := []systemCommandTest{
		{args: `{"unexpected": "not a valid key"}`, response: "system_command: error: missing command"},
		{args: `{command": "cat junk"}`, response: "system_command: error: invalid character"},
		{args: `{"command": "git"}`, response: "system_command: error: command not allowed"},
		{args: `{"command": "cat junk"}`, response: "No such file or directory"},
		{args: `{"command": "python3 --version"}`, response: "system_command: error: command not allowed"},
	}

	for _, ct := range cmdTests {
		resp := systemCommand(ct.args)

		if !strings.Contains(resp, ct.response) {
			t.Fatalf("Expected response `%s`, received `%s`", ct.response, resp)
		}
	}
}
