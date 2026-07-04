package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"larkspur"

	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

var (
	versionId           = "v0.1.0"
	exitCodeNoError     = 0
	exitCodeNoLogFile   = 1
	exitCodeBadProvider = 2
	exitCodeNoProvider  = 3
	quitStrings = []string{"/quit", "/q", "/exit"}
)

// usage displays the command line usage
func usage() {
	w := flag.CommandLine.Output()

	fmt.Fprintf(w, "Usage: larkspur [options]\n")
	flag.PrintDefaults()
}

func shouldExit(prompt string) bool {
	for _, quit := range quitStrings {
		if prompt == quit {
			return true
		}
	}

	return false
}

func main() {
	// Define our command line flags
	var level string
	var version bool

	flag.StringVar(&level, "level", "ERROR", "Set the logging level [ERROR, WARN, INFO, DEGUG]")
	flag.BoolVar(&version, "v", false, "Display the product version.")

	// Define our usage statement
	flag.Usage = usage

	// Parse our flags
	flag.Parse()

	// Show version if requested
	if version {
		fmt.Printf("Version: %s\n", versionId)
		os.Exit(exitCodeNoError)
	}

	configureLogger(level)
	provider := getProvider()

	for {
		fmt.Printf("User: ")

		reader := bufio.NewReader(os.Stdin)
		prompt, err := reader.ReadString('\n')
		if err != nil {
			prompt = ""
		}

		prompt = strings.TrimSuffix(prompt, "\n")

		if shouldExit(prompt) {
			break
		}

		if prompt != "" {
			route := larkspur.Route(provider.(*ollama.Provider), prompt)

			fmt.Printf("Route: %s\n", route)

			if route != "" {
				response := larkspur.Chat(provider.(*ollama.Provider), route)

				fmt.Printf("Agent 🥳: %s\n", response)
				fmt.Println()
				fmt.Println()
			}
		}
	}
}
