package memory

import (
	"errors"
	"path/filepath"
	"testing"
)

func TestStore(t *testing.T) {
	t.Run("Testing Put and Get", testPutAndGet)
	t.Run("Testing Get missing key", testGetMissingKey)
	t.Run("Testing empty key", testEmptyKey)
	t.Run("Testing overwrite", testOverwrite)
	t.Run("Testing Delete", testDelete)
	t.Run("Testing List", testList)
	t.Run("Testing Search", testSearch)
}

func newTestStore(t *testing.T) *Store {
	path := filepath.Join(t.TempDir(), "memories.db")

	store, err := NewStore(path)
	if err != nil {
		t.Fatalf("could not NewStore: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
	})

	return store
}

func testPutAndGet(t *testing.T) {
	store := newTestStore(t)

	if err := store.Put("objective", "refactor the planner"); err != nil {
		t.Fatalf("could not Put: %v", err)
	}

	value, err := store.Get("objective")
	if err != nil {
		t.Fatalf("could not Get: %v", err)
	}

	if value != "refactor the planner" {
		t.Fatalf("Expected `refactor the planner`, received `%s`", value)
	}
}

func testGetMissingKey(t *testing.T) {
	store := newTestStore(t)

	_, err := store.Get("missing")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Expected ErrNotFound, received `%v`", err)
	}
}

func testEmptyKey(t *testing.T) {
	store := newTestStore(t)

	if err := store.Put("", "value"); !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("Expected ErrEmptyKey from Put, received `%v`", err)
	}

	if _, err := store.Get(""); !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("Expected ErrEmptyKey from Get, received `%v`", err)
	}

	if err := store.Delete(""); !errors.Is(err, ErrEmptyKey) {
		t.Fatalf("Expected ErrEmptyKey from Delete, received `%v`", err)
	}
}

func testOverwrite(t *testing.T) {
	store := newTestStore(t)

	store.Put("key", "first")
	store.Put("key", "second")

	value, err := store.Get("key")
	if err != nil {
		t.Fatalf("could not Get: %v", err)
	}

	if value != "second" {
		t.Fatalf("Expected `second`, received `%s`", value)
	}
}

func testDelete(t *testing.T) {
	store := newTestStore(t)

	store.Put("key", "value")

	if err := store.Delete("key"); err != nil {
		t.Fatalf("could not Delete: %v", err)
	}

	_, err := store.Get("key")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Expected ErrNotFound after Delete, received `%v`", err)
	}

	// Deleting a key that no longer exists is not an error.
	if err := store.Delete("key"); err != nil {
		t.Fatalf("could not Delete missing key: %v", err)
	}
}

func testList(t *testing.T) {
	store := newTestStore(t)

	store.Put("a", "1")
	store.Put("b", "2")

	memories, err := store.List()
	if err != nil {
		t.Fatalf("could not List: %v", err)
	}

	if len(memories) != 2 {
		t.Fatalf("Expected 2 memories, received %d", len(memories))
	}

	if memories["a"] != "1" || memories["b"] != "2" {
		t.Fatalf("Expected {a:1 b:2}, received %v", memories)
	}
}

func testSearch(t *testing.T) {
	store := newTestStore(t)

	store.Put("color", "blue")
	store.Put("shape", "circle")

	matches, err := store.Search("colo")
	if err != nil {
		t.Fatalf("could not Search: %v", err)
	}

	if len(matches) != 1 || matches["color"] != "blue" {
		t.Fatalf("Expected {color:blue}, received %v", matches)
	}

	matches, err = store.Search("blu")
	if err != nil {
		t.Fatalf("could not Search: %v", err)
	}

	if len(matches) != 1 || matches["color"] != "blue" {
		t.Fatalf("Expected value match to find {color:blue}, received %v", matches)
	}

	matches, err = store.Search("")
	if err != nil {
		t.Fatalf("could not Search: %v", err)
	}

	if len(matches) != 2 {
		t.Fatalf("Expected empty query to match all 2 memories, received %d", len(matches))
	}

	matches, err = store.Search("no-such-thing")
	if err != nil {
		t.Fatalf("could not Search: %v", err)
	}

	if len(matches) != 0 {
		t.Fatalf("Expected no matches, received %v", matches)
	}
}
