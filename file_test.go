package larkspur

import (
	"fmt"
	"strings"
	"testing"
)

type fileTest struct {
	args     string
	response string
}

func TestFile(t *testing.T) {
	t.Run("Testing fileReadFull", testFileReadFull)
	t.Run("Testing fileWriteFull", testFileWriteFull)
	t.Run("Testing fileReadLines", testFileReadLines)
}

func testFileReadFull(t *testing.T) {
	fmt.Println(t.Name())

	readTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_read_full: error: missing file name"},
		{args: `{name": "data/good_file.txt"}`, response: "file_read_full: error: invalid character"},
		{args: `{"name": "data/bad_file.txt"}`, response: "file_read_full: error: "},
		{args: `{"name": "data/good_file.txt"}`, response: "This is some good\ndata."},
	}

	for _, test := range readTests {
		response := fileReadFull(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}
}

func testFileWriteFull(t *testing.T) {
	fmt.Println(t.Name())

	writeTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_write_full: error: missing file name"},
		{args: `{name": "data/write_file.txt", "content": "Wrote some data."}`, response: "file_write_full: error: invalid character"},
		{args: `{"name": "data/write_file.txt", "content": "Wrote some data."}`, response: "file_write_full: success"},
	}

	for _, test := range writeTests {
		response := fileWriteFull(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}
}

func testFileReadLines(t *testing.T) {
	fmt.Println(t.Name())

	readTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_read_lines: error: missing file name"},
		{args: `{name": "data/lined_file.txt", "start": 1, "stop": 1}`, response: "file_read_lines: error: invalid character"},
		{args: `{"name": "data/lined_file.txt", "start": 1, "stop": 1}`, response: "This is line 1"},
		{args: `{"name": "data/lined_file.txt", "start": 9, "stop": 10}`, response: ""},
		{args: `{"name": "data/lined_file.txt", "start": 1, "stop": 7}`, response: "This is line 1\nThis is line 2\nThis is line 3\n\nThis is line 5\nThis is line 6\n\n"},
		{args: `{"name": "data/lined_file.txt", "start": 1, "stop": 8}`, response: "This is line 1\nThis is line 2\nThis is line 3\n\nThis is line 5\nThis is line 6\n\n"},
		{args: `{"name": "data/lined_file.txt", "start": 3, "stop": 5}`, response: "This is line 3\n\nThis is line 5"},
	}

	for _, test := range readTests {
		response := fileReadLines(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}
}
