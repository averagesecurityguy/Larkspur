package larkspur

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"larkspur/memory"
)

func TestMemory(t *testing.T) {
	store, err := memory.NewStore(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatalf("could not NewStore: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		SetMemoryStore(nil)
	})

	SetMemoryStore(store)

	t.Run("Testing memoryPut and memoryGet", testMemoryPutAndGet)
	t.Run("Testing memoryGet missing key", testMemoryGetMissing)
	t.Run("Testing memorySearch", testMemorySearch)
	t.Run("Testing storeMemory with no store", testStoreMemoryNoStore)
}

func testMemoryPutAndGet(t *testing.T) {
	fmt.Println(t.Name())

	put := memoryPut(`{"key": "objective", "value": "refactor the planner"}`)
	if put != "memory_put: success" {
		t.Fatalf("Expected `memory_put: success`, received `%s`", put)
	}

	got := memoryGet(`{"key": "objective"}`)
	if got != "refactor the planner" {
		t.Fatalf("Expected `refactor the planner`, received `%s`", got)
	}
}

func testMemoryGetMissing(t *testing.T) {
	fmt.Println(t.Name())

	got := memoryGet(`{"key": "does-not-exist"}`)
	if got != "memory_get: not found" {
		t.Fatalf("Expected `memory_get: not found`, received `%s`", got)
	}
}

func testMemorySearch(t *testing.T) {
	fmt.Println(t.Name())

	memoryPut(`{"key": "color", "value": "blue"}`)

	got := memorySearch(`{"query": "colo"}`)
	if !strings.Contains(got, "color: blue") {
		t.Fatalf("Expected result to contain `color: blue`, received `%s`", got)
	}

	got = memorySearch(`{"query": "no-such-thing"}`)
	if got != "No matching memories" {
		t.Fatalf("Expected `No matching memories`, received `%s`", got)
	}
}

func testStoreMemoryNoStore(t *testing.T) {
	fmt.Println(t.Name())

	SetMemoryStore(nil)
	defer SetMemoryStore(nil)

	if err := storeMemory("key", "value"); err != nil {
		t.Fatalf("Expected storeMemory to no-op without a store, received `%v`", err)
	}
}
