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
