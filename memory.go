package larkspur

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"larkspur/memory"

	"github.com/rs/zerolog/log"
)

// memoryStore is the process-wide store backing the memory_* tools and the
// summarizer's memory write-back. It must be installed with SetMemoryStore
// before any agent invokes those tools.
var memoryStore *memory.Store

// SetMemoryStore installs the store used by the memory_* tools and by
// SummarizePlanResults to read and write persisted agent memories.
func SetMemoryStore(store *memory.Store) {
	memoryStore = store
}

// storeMemory persists a key/value memory using the process-wide store. It
// is a no-op, rather than an error, when no store has been installed, so
// callers like SummarizePlanResults still work in contexts where memory
// hasn't been wired up.
func storeMemory(key, value string) error {
	if memoryStore == nil {
		return nil
	}

	return memoryStore.Put(key, value)
}

// checkpointKeyPrefix namespaces checkpoint memories in the store so they
// are easy to distinguish from durable, cross-session memories and can be
// cleaned up once a plan finishes.
const checkpointKeyPrefix = "checkpoint:"

// checkpointKey returns the memory key used to store planID's checkpoint.
func checkpointKey(planID string) string {
	return checkpointKeyPrefix + planID
}

// getCheckpoint returns the current checkpoint for planID, or "" if none has
// been recorded yet, planID is empty, or no memory store is available.
func getCheckpoint(planID string) string {
	if planID == "" || memoryStore == nil {
		return ""
	}

	value, err := memoryStore.Get(checkpointKey(planID))
	if err != nil {
		return ""
	}

	return value
}

// putCheckpoint overwrites planID's checkpoint with value. It is a no-op
// when planID is empty or no memory store is available, the same tolerance
// storeMemory has for those contexts.
func putCheckpoint(planID, value string) {
	if planID == "" || memoryStore == nil {
		return
	}

	if err := memoryStore.Put(checkpointKey(planID), value); err != nil {
		log.Error().Err(err).Str("planID", planID).Msg("could not write checkpoint")
	}
}

// clearCheckpoint removes planID's checkpoint once its plan has finished, so
// it does not linger and surface in memory_search results for unrelated
// future plans.
func clearCheckpoint(planID string) {
	if planID == "" || memoryStore == nil {
		return
	}

	if err := memoryStore.Delete(checkpointKey(planID)); err != nil {
		log.Error().Err(err).Str("planID", planID).Msg("could not clear checkpoint")
	}
}

type taskCheckpointArgs struct {
	NextStep string `json:"next_step"`
}

// taskCheckpoint records the agent's own account of what to do next for
// planID, so a fresh Chat loop (the next checklist item, or the loop that
// follows a history compaction) can remind the model where it left off
// instead of losing the thread.
func taskCheckpoint(planID, arguments string) string {
	var args taskCheckpointArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("task_checkpoint: error: %v", err)
	}

	if args.NextStep == "" {
		log.Error().Msg("missing next_step")
		return fmt.Sprintf("task_checkpoint: error: missing next_step")
	}

	if planID == "" || memoryStore == nil {
		return fmt.Sprintf("task_checkpoint: error: checkpoint not available")
	}

	putCheckpoint(planID, args.NextStep)

	return "task_checkpoint: success"
}

// noteCheckpointAction overwrites planID's checkpoint with a mechanical
// breadcrumb describing the tool call that was just made. It runs after
// every non-checkpoint tool call in the ReAct loop so there is always a
// checkpoint to recover from even if the agent never calls task_checkpoint
// itself.
func noteCheckpointAction(planID, toolName, arguments string) {
	putCheckpoint(planID, fmt.Sprintf("last action: called %s with %s", toolName, arguments))
}

type memoryGetArgs struct {
	Key string `json:"key"`
}

type memorySearchArgs struct {
	Query string `json:"query"`
}

type memoryPutArgs struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// memoryGet retrieves the memory stored under the given key.
func memoryGet(arguments string) string {
	var args memoryGetArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("memory_get: error: %v", err)
	}

	if args.Key == "" {
		log.Error().Msg("missing key")
		return fmt.Sprintf("memory_get: error: missing key")
	}

	if memoryStore == nil {
		return fmt.Sprintf("memory_get: error: memory store not available")
	}

	value, err := memoryStore.Get(args.Key)
	if err != nil {
		if errors.Is(err, memory.ErrNotFound) {
			return "memory_get: not found"
		}

		log.Error().Err(err).Msg("could not read memory")
		return fmt.Sprintf("memory_get: error: could not read memory")
	}

	return value
}

// memorySearch finds memories whose key or value contains the given query,
// case-insensitively. An empty query returns every stored memory.
func memorySearch(arguments string) string {
	var args memorySearchArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("memory_search: error: %v", err)
	}

	if memoryStore == nil {
		return fmt.Sprintf("memory_search: error: memory store not available")
	}

	matches, err := memoryStore.Search(args.Query)
	if err != nil {
		log.Error().Err(err).Msg("could not search memories")
		return fmt.Sprintf("memory_search: error: could not search memories")
	}

	if len(matches) == 0 {
		return "No matching memories"
	}

	keys := make([]string, 0, len(matches))
	for key := range matches {
		keys = append(keys, key)
	}

	slices.Sort(keys)

	var result strings.Builder
	for _, key := range keys {
		fmt.Fprintf(&result, "%s: %s\n", key, matches[key])
	}

	return result.String()
}

// memoryPut stores value under key, overwriting any existing value for that
// key.
func memoryPut(arguments string) string {
	var args memoryPutArgs

	err := json.Unmarshal([]byte(arguments), &args)
	if err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("memory_put: error: %v", err)
	}

	if args.Key == "" {
		log.Error().Msg("missing key")
		return fmt.Sprintf("memory_put: error: missing key")
	}

	if memoryStore == nil {
		return fmt.Sprintf("memory_put: error: memory store not available")
	}

	err = memoryStore.Put(args.Key, args.Value)
	if err != nil {
		log.Error().Err(err).Msg("could not write memory")
		return fmt.Sprintf("memory_put: error: could not write memory")
	}

	return "memory_put: success"
}
