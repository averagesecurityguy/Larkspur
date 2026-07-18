package main

import (
	"context"
	"fmt"
	"os"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

func getProvider() anyllm.Provider {
	provider, err := ollama.New(anyllm.WithTimeout(900 * time.Second))
	if err != nil {
		fmt.Printf("Bad provider config: %v\n", err)
		os.Exit(exitCodeBadProvider)
	}

	_, err = provider.ListModels(context.Background())
	if err != nil {
		fmt.Printf("Provider not available: %v\n", err)
		os.Exit(exitCodeNoProvider)
	}

	return provider
}
