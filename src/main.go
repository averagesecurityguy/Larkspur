package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"larkspur"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

func main() {
	provider, err := ollama.New(anyllm.WithTimeout(300 * time.Second))
	if err != nil {
		log.Fatal(err)
	}

	// Use the model name from the server, or fallback to a default.
	modelName := "qwen3.5:2b"

	for {
		fmt.Printf("User: ")

		reader := bufio.NewReader(os.Stdin)
		prompt, err := reader.ReadString('\n')
		if err != nil {
			prompt = ""
		}

		prompt = strings.TrimSuffix(prompt, "\n")

		fmt.Printf("  Prompt: `%v`", prompt)

		if prompt != "" {
			response := larkspur.Chat(provider, modelName, prompt)

			fmt.Printf("Agent 🥳: %s\n", response)
			fmt.Println()
			fmt.Println()
		}
	}
}
