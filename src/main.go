package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"larkspur"
	"larkspur/memory"
)

var (
	versionId              = "v0.1.0"
	exitCodeNoError        = 0
	exitCodeNoLogFile      = 1
	exitCodeBadProvider    = 2
	exitCodeNoProvider     = 3
	exitCodeBadLogFile     = 4
	exitCodeNoHomeDir      = 5
	exitCodeBadMemoryStore = 6
	quitStrings            = []string{"/quit", "/q", "/exit"}
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
	var logFileName string
	var answer string
	var larkContext string

	flag.StringVar(&level, "level", "ERROR", "Set the logging level [ERROR, WARN, INFO, DEBUG]")
	flag.BoolVar(&version, "v", false, "Display the product version.")
	flag.StringVar(&logFileName, "log", "larkspur.log", "Name of file where logs will be written.")

	// Define our usage statement
	flag.Usage = usage

	// Parse our flags
	flag.Parse()

	// Show version if requested
	if version {
		fmt.Printf("Version: %s\n", versionId)
		os.Exit(exitCodeNoError)
	}

	logFile := openLogFile(logFileName)
	defer logFile.Close()

	configureLogger(level, logFile)

	memStore, err := memory.NewStore(memoriesDBPath())
	if err != nil {
		fmt.Printf("Could not open memory store: %v\n", err)
		os.Exit(exitCodeBadMemoryStore)
	}
	defer memStore.Close()

	larkspur.SetMemoryStore(memStore)

	provider := getProvider()

	// reader is created once, outside the loop. Recreating it on every
	// iteration would discard any input already buffered internally past the
	// first line read from a single underlying read, silently losing lines
	// (e.g. a piped "/quit") and leaving the loop spinning on EOF forever
	// once stdin is exhausted.
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Printf("User: ")

		// Stop once stdin is exhausted (EOF) or unreadable, rather than
		// spinning forever re-printing the prompt.
		if !scanner.Scan() {
			break
		}

		prompt := scanner.Text()
		prompt = strings.TrimSuffix(prompt, "\n")

		if shouldExit(prompt) {
			break
		}

		if prompt != "" {
			answer, larkContext = larkspur.AnswerPrompt(context.Background(), provider, prompt, larkContext)
			fmt.Printf("Lark: %s\n", answer)
		}
	}
}
