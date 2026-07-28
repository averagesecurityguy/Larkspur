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
	t.Run("Testing checkpoint lifecycle", testCheckpointLifecycle)
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

func testCheckpointLifecycle(t *testing.T) {
	fmt.Println(t.Name())

	planID := "plan-123"

	if got := getCheckpoint(planID); got != "" {
		t.Fatalf("Expected no checkpoint yet, received `%s`", got)
	}

	missing := taskCheckpoint(planID, `{"next_step": ""}`)
	if !strings.Contains(missing, "error") {
		t.Fatalf("Expected an error for a missing next_step, received `%s`", missing)
	}

	set := taskCheckpoint(planID, `{"next_step": "write the tests"}`)
	if set != "task_checkpoint: success" {
		t.Fatalf("Expected `task_checkpoint: success`, received `%s`", set)
	}

	if got := getCheckpoint(planID); got != "write the tests" {
		t.Fatalf("Expected `write the tests`, received `%s`", got)
	}

	noteCheckpointAction(planID, "file_read_full", `{"file_name": "main.go"}`)

	got := getCheckpoint(planID)
	if !strings.Contains(got, "file_read_full") {
		t.Fatalf("Expected breadcrumb to mention file_read_full, received `%s`", got)
	}

	clearCheckpoint(planID)

	if got := getCheckpoint(planID); got != "" {
		t.Fatalf("Expected checkpoint to be cleared, received `%s`", got)
	}

	// A checkpoint scoped to no plan is always empty and never stored.
	if got := getCheckpoint(""); got != "" {
		t.Fatalf("Expected no checkpoint for an empty planID, received `%s`", got)
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
