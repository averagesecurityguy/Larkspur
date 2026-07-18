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
	exitCodeBadLogFile  = 4
	quitStrings         = []string{"/quit", "/q", "/exit"}
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
			// Keep track of our context throughout the execution of the objective.
			var oc strings.Builder
			var final string

			// Build a plan
			plan, err := larkspur.GeneratePlan(provider.(*ollama.Provider), prompt)
			if err != nil {
				fmt.Println("Agent ☹️: I was unable to create a plan. Please make your request again.")
				continue
			}

			// Execute our plan one task at a time.
			fmt.Printf("Agent 🫡: %s\n", plan.Objective)
			oc.WriteString(fmt.Sprintf("Objective: %s\n", plan.Objective))

			for i, goal := range plan.Goals {
				// Keep track of our context throughout the execution of a particular goal.
				var gc strings.Builder

				// Add our objective context to the goal context
				gc.WriteString(oc.String())
				gc.WriteString(fmt.Sprintf("Goal %d: %s\n", i, goal.Goal))

				// Execute all of the tasks for this goal.
				for _, task := range goal.TaskList {
					fmt.Printf("%s\n", task)
					final = task.Execute(provider, gc.String())
					gc.WriteString(final)
				}

				// Summarize the single goal context into the objective context.
				oc.WriteString(fmt.Sprintf("Goal %d of %d: %s\n", i, len(plan.Goals), goal.Goal))

				resp, err := larkspur.Chat(provider.(*ollama.Provider), "summarizer", gc.String(), "")
				if err != nil {
					continue
				}
				oc.WriteString(fmt.Sprintf("%s\n", resp))
			}

			fmt.Printf("Agent 🥳: %s\n", final)
		}
	}
}
