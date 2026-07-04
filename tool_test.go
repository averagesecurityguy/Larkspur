package larkspur

import (
	"fmt"
	"slices"
	"testing"
)

type toolTest struct {
	path  string
	names []string
}

func TestTool(t *testing.T) {
	t.Run("Testing loadTools", testLoadTools)
	t.Run("Testing LoadAllTools", testLoadAllTools)
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
	}
	tools := LoadAllTools()
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
