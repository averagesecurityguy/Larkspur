package larkspur

import (
	"fmt"
	"os"
	"path/filepath"
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
	t.Run("Testing fileEdit", testFileEdit)
	t.Run("Testing fileDelete", testFileDelete)
	t.Run("Testing fileMove", testFileMove)
	t.Run("Testing dirCreate", testDirCreate)
	t.Run("Testing dirList", testDirList)
}

func testFileReadFull(t *testing.T) {
	fmt.Println(t.Name())

	readTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_read_full: error: missing file name"},
		{args: `{file_name": "data/good_file.txt"}`, response: "file_read_full: error: invalid character"},
		{args: `{"file_name": "data/bad_file.txt"}`, response: "file_read_full: error: "},
		{args: `{"file_name": "data/good_file.txt"}`, response: "This is some good\ndata."},
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
		{args: `{file_name": "data/write_file.txt", "content": "Wrote some data."}`, response: "file_write_full: error: invalid character"},
		{args: `{"file_name": "data/write_file.txt", "content": "Wrote some data."}`, response: "file_write_full: success"},
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
		{args: `{file_name": "data/lined_file.txt", "start_line": 1, "stop_line": 1}`, response: "file_read_lines: error: invalid character"},
		{args: `{"file_name": "data/lined_file.txt", "start_line": 1, "stop_line": 1}`, response: "This is line 1"},
		{args: `{"file_name": "data/lined_file.txt", "start_line": 9, "stop_line": 10}`, response: ""},
		{args: `{"file_name": "data/lined_file.txt", "start_line": 1, "stop_line": 7}`, response: "This is line 1\nThis is line 2\nThis is line 3\n\nThis is line 5\nThis is line 6\n\n"},
		{args: `{"file_name": "data/lined_file.txt", "start_line": 1, "stop_line": 8}`, response: "This is line 1\nThis is line 2\nThis is line 3\n\nThis is line 5\nThis is line 6\n\n"},
		{args: `{"file_name": "data/lined_file.txt", "start_line": 3, "stop_line": 5}`, response: "This is line 3\n\nThis is line 5"},
	}

	for _, test := range readTests {
		response := fileReadLines(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}
}

func testFileEdit(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()
	path := filepath.Join(dir, "edit.txt")

	if err := os.WriteFile(path, []byte("one fish\ntwo fish\nred fish\nblue fish\n"), 0644); err != nil {
		t.Fatalf("could not WriteFile: %v", err)
	}

	editTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_edit: error: missing file name"},
		{args: fmt.Sprintf(`{"file_name": %q, "new_string": "x"}`, path), response: "file_edit: error: missing old_string"},
		{args: fmt.Sprintf(`{"file_name": %q, "old_string": "x", "new_string": "x"}`, path), response: "file_edit: error: old_string and new_string are identical"},
		{args: fmt.Sprintf(`{"file_name": %q, "old_string": "no such text", "new_string": "x"}`, path), response: "file_edit: error: old_string not found"},
		{args: fmt.Sprintf(`{"file_name": %q, "old_string": "fish", "new_string": "cat"}`, path), response: "appears 4 times"},
		{args: fmt.Sprintf(`{"file_name": %q, "old_string": "red fish", "new_string": "green cat"}`, path), response: "file_edit: success"},
	}

	for _, test := range editTests {
		response := fileEdit(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not ReadFile: %v", err)
	}
	if got := string(data); !strings.Contains(got, "green cat") || strings.Contains(got, "red fish") {
		t.Fatalf("Expected the single occurrence to be replaced, received `%s`", got)
	}

	response := fileEdit(fmt.Sprintf(`{"file_name": %q, "old_string": "fish", "new_string": "cat", "replace_all": true}`, path))
	if !strings.Contains(response, "file_edit: success (replaced 3 occurrences)") {
		t.Fatalf("Expected a replace_all success message, received `%s`", response)
	}

	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not ReadFile: %v", err)
	}
	if strings.Contains(string(data), "fish") {
		t.Fatalf("Expected every occurrence to be replaced, received `%s`", string(data))
	}
}

func testFileDelete(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()
	path := filepath.Join(dir, "delete.txt")

	if err := os.WriteFile(path, []byte("gone soon"), 0644); err != nil {
		t.Fatalf("could not WriteFile: %v", err)
	}

	deleteTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_delete: error: missing file name"},
		{args: fmt.Sprintf(`{"file_name": %q}`, filepath.Join(dir, "nope")), response: "file_delete: error: could not find file"},
		{args: fmt.Sprintf(`{"file_name": %q}`, dir), response: "is a directory, not a file"},
		{args: fmt.Sprintf(`{"file_name": %q}`, path), response: "file_delete: success"},
	}

	for _, test := range deleteTests {
		response := fileDelete(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}

	if _, err := os.Stat(path); err == nil {
		t.Fatalf("Expected %s to no longer exist", path)
	}
}

func testFileMove(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()
	src := filepath.Join(dir, "source.txt")
	dst := filepath.Join(dir, "destination.txt")

	if err := os.WriteFile(src, []byte("moving"), 0644); err != nil {
		t.Fatalf("could not WriteFile: %v", err)
	}

	moveTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "file_move: error: missing source or destination"},
		{args: fmt.Sprintf(`{"source": %q, "destination": %q}`, filepath.Join(dir, "nope"), dst), response: "file_move: error: could not move"},
		{args: fmt.Sprintf(`{"source": %q, "destination": %q}`, src, dst), response: "file_move: success"},
	}

	for _, test := range moveTests {
		response := fileMove(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}

	if _, err := os.Stat(src); err == nil {
		t.Fatalf("Expected %s to no longer exist", src)
	}

	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", dst, err)
	}
	if string(data) != "moving" {
		t.Fatalf("Expected `moving`, received `%s`", string(data))
	}
}

func testDirCreate(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c")

	createTests := []fileTest{
		{args: `{"unexpected": "not a valid key"}`, response: "dir_create: error: missing path"},
		{args: fmt.Sprintf(`{"path": %q}`, target), response: "dir_create: success"},
		{args: fmt.Sprintf(`{"path": %q}`, target), response: "dir_create: success"},
	}

	for _, test := range createTests {
		response := dirCreate(test.args)
		if !strings.Contains(response, test.response) {
			t.Fatalf("Expected `%s`, received `%s`", test.response, response)
		}
	}

	info, err := os.Stat(target)
	if err != nil || !info.IsDir() {
		t.Fatalf("Expected %s to be a directory", target)
	}
}

func testDirList(t *testing.T) {
	fmt.Println(t.Name())

	dir := t.TempDir()
	empty := filepath.Join(dir, "empty")

	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatalf("could not MkdirAll: %v", err)
	}

	response := dirList(fmt.Sprintf(`{"path": %q}`, filepath.Join(dir, "nope")))
	if !strings.Contains(response, "dir_list: error") {
		t.Fatalf("Expected an error for a missing directory, received `%s`", response)
	}

	response = dirList(fmt.Sprintf(`{"path": %q}`, empty))
	if response != "<empty directory>" {
		t.Fatalf("Expected `<empty directory>`, received `%s`", response)
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("12345"), 0644); err != nil {
		t.Fatalf("could not WriteFile: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "subdir"), 0755); err != nil {
		t.Fatalf("could not MkdirAll: %v", err)
	}

	response = dirList(fmt.Sprintf(`{"path": %q}`, dir))
	if !strings.Contains(response, "file\t5\tfile.txt") {
		t.Fatalf("Expected a file entry with its size, received `%s`", response)
	}
	if !strings.Contains(response, "dir\t-\tsubdir") {
		t.Fatalf("Expected a directory entry, received `%s`", response)
	}
}
