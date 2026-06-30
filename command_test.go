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
	command  string
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

		{command: "cat junk >> junk.txt", response: "No such file or directory"},
		{command: "python3 --version", response: "Success"},
	}

	for _, ct := range cmdTests {
		resp := SystemCommand(ct.command)

		if !strings.Contains(resp, ct.response) {
			t.Fatalf("Expected response `%s`, received `%s`", ct.response, resp)
		}
	}
}
