package larkspur

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

type toolTest struct {
	path  string
	names []string
}

func TestTool(t *testing.T) {
	t.Run("Testing loadTools", testLoadTools)
	t.Run("Testing LoadAllTools", testLoadAllTools)
	t.Run("Testing loadExecutionTools", testLoadExecutionTools)
	t.Run("Testing truncateToolOutput", testTruncateToolOutput)
}

func testLoadTools(t *testing.T) {
	fmt.Println(t.Name())

	loadTests := []toolTest{
		{path: "data/not_exist.json", names: []string{}},
		{path: "data/bad_tool.json", names: []string{}},
		{path: "data/good_tool.json", names: []string{"command1", "command2"}},
	}

	for _, test := range loadTests {
		tools := loadTools(test.path)
		names := []string{}

		for _, tool := range tools {
			names = append(names, tool.Function.Name)
		}

		slices.Sort(names)
		slices.Sort(test.names)

		if !slices.Equal(test.names, names) {
			t.Fatalf("Expected `%v`, received `%v`", test.names, names)
		}
	}
}

func testLoadAllTools(t *testing.T) {
	fmt.Println(t.Name())

	allNames := []string{
		"system_command",
		"file_write_full",
		"file_read_full",
		"file_read_lines",
		"file_size_bytes",
		"file_size_lines",
		"file_find_glob",
		"memory_search",
		"memory_get",
		"memory_put",
	}
	tools := loadAllTools()
	names := []string{}

	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}

	slices.Sort(names)
	slices.Sort(allNames)

	if !slices.Equal(allNames, names) {
		t.Fatalf("Expected `%v`, received `%v`", allNames, names)
	}
}

func testLoadExecutionTools(t *testing.T) {
	fmt.Println(t.Name())

	allNames := []string{
		"system_command",
		"file_write_full",
		"file_read_full",
		"file_read_lines",
		"file_size_bytes",
		"file_size_lines",
		"file_find_glob",
		"memory_search",
		"memory_get",
		"memory_put",
		"task_checkpoint",
	}
	tools := loadExecutionTools()
	names := []string{}

	for _, tool := range tools {
		names = append(names, tool.Function.Name)
	}

	slices.Sort(names)
	slices.Sort(allNames)

	if !slices.Equal(allNames, names) {
		t.Fatalf("Expected `%v`, received `%v`", allNames, names)
	}
}

func testTruncateToolOutput(t *testing.T) {
	fmt.Println(t.Name())

	testAgent := &agent{contextTokens: defaultModelContextTokens}

	short := "short output"
	if got := truncateToolOutput(testAgent, "test_tool", short); got != short {
		t.Fatalf("Expected `%s`, received `%s`", short, got)
	}

	limit := testAgent.maxToolOutputChars()
	long := strings.Repeat("a", limit+100)
	got := truncateToolOutput(testAgent, "test_tool", long)

	if !strings.HasPrefix(got, long[:limit]) {
		t.Fatalf("Expected truncated output to start with the first %d characters of the original", limit)
	}

	if !strings.Contains(got, "truncated") {
		t.Fatalf("Expected truncated output to include a truncation note, received `%s`", got)
	}
}
