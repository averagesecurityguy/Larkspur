package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/openai"

	"larkspur"
)

const (
	// defaultOllamaHost mirrors the ollama provider's own default so the
	// OpenAI-compatible endpoint is reachable without any extra
	// configuration. OLLAMA_HOST overrides it, same as the native provider.
	defaultOllamaHost = "http://localhost:11434"

	// ollamaAPIKey is a placeholder. The openai provider requires a
	// non-empty key, but ollama's OpenAI-compatible endpoint doesn't check
	// it, so any value works.
	ollamaAPIKey = "ollama"
)

func getProvider() anyllm.Provider {
	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = defaultOllamaHost
	}
	baseURL := strings.TrimRight(host, "/") + "/v1"

	provider, err := openai.New(
		anyllm.WithBaseURL(baseURL),
		anyllm.WithAPIKey(ollamaAPIKey),
		anyllm.WithTimeout(900*time.Second),
	)
	if err != nil {
		fmt.Printf("Bad provider config: %v\n", err)
		os.Exit(exitCodeBadProvider)
	}

	_, err = provider.ListModels(context.Background())
	if err != nil {
		fmt.Printf("Provider not available: %v\n", err)
		os.Exit(exitCodeNoProvider)
	}

	// Query each agent's actual model context length from ollama's native
	// API (host, not baseURL: this doesn't go through the OpenAI-compatible
	// endpoint) so the shared compaction thresholds are sized off real
	// values instead of the generic default. Best-effort: a query failure
	// logs a warning inside RefreshAgentContextWindows and leaves that
	// agent at its previous value rather than blocking startup.
	if err := larkspur.RefreshAgentContextWindows(host); err != nil {
		fmt.Printf("Could not query model context windows: %v\n", err)
	}

	return provider
}
