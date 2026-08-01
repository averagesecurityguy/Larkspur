//go:build e2e

package larkspur

// e2e_test defines a small suite of prompts that exercise larkspur's full
// pipeline (plan creation, objective execution, checklist verification, and
// summarization) against a live ollama provider. Unlike the rest of this
// package's tests, these hit the provider and a real model on purpose, so
// they're gated behind the "e2e" build tag and excluded from `go test
// ./...`. Run them explicitly:
//
//	go test -tags e2e -timeout 60m -run TestEndToEnd -v .
//
// ollama must be running locally with the model configured in agents.go
// (gemma4:e2b) pulled. If it isn't reachable, the suite skips instead of
// failing. Assertions check observable filesystem state (what the agent's
// tools actually produced), not the model's prose, since that varies run to
// run. Even so, a small local model can occasionally fail a task outright,
// or simply take an unreasonably long time to respond to one particular
// completion call — that's the exact unreliability the checkpoint system
// exists to mitigate, not a guarantee these tests eliminate. A failure here
// is worth a rerun before it's treated as a regression.
//
// Each subtest gets its own perTestTimeout, independent of the others, via
// a context.Context passed all the way down to chat()'s own
// provider.Completion calls (see chat.go): a stall in Easy or Medium
// doesn't eat into Hard's budget, and a stall in any one of them fails that
// subtest cleanly on its own deadline — rather than every subtest sharing a
// single overall `go test -timeout`, where a stall anywhere kills the
// entire binary via a panic and loses the results of every subtest that
// already passed. The -timeout above just needs to be comfortably larger
// than perTestTimeout × (number of subtests), as a backstop.
//
// TestMain configures the logger at debug level, writing every chat prompt,
// response, and tool call to e2e_test.log (in this package's directory) so
// a run can be reviewed after the fact — including which agent handled each
// step, which matters here since checklist verification and objective
// execution now go through different agents.

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/openai"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"larkspur/memory"
)

// e2eOllamaAPIKey is a placeholder. The openai provider requires a
// non-empty key, but ollama's OpenAI-compatible endpoint doesn't check it,
// so any value works. Mirrors src/provider.go's getProvider.
const e2eOllamaAPIKey = "ollama"

// perTestTimeout bounds a single subtest's entire runPlan call, independent
// of the other subtests. Generous on purpose: a small local model can
// legitimately take minutes on one completion call, and this only needs to
// catch the case where it never returns at all.
const perTestTimeout = 15 * time.Minute

// TestMain configures debug-level logging to e2e_test.log before any test
// in this suite runs, then restores the exit code go test expects.
func TestMain(m *testing.M) {
	logFile, err := os.OpenFile("e2e_test.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("could not open e2e_test.log: %v\n", err)
		os.Exit(1)
	}

	zerolog.SetGlobalLevel(zerolog.DebugLevel)
	zerolog.TimestampFunc = time.Now().UTC
	log.Logger = zerolog.New(logFile).With().Timestamp().Caller().Logger()

	fmt.Println("e2e: debug logs writing to e2e_test.log")

	code := m.Run()

	logFile.Close()
	os.Exit(code)
}

func TestEndToEnd(t *testing.T) {
	provider := newTestProvider(t)

	store, err := memory.NewStore(filepath.Join(t.TempDir(), "memories.db"))
	if err != nil {
		t.Fatalf("could not NewStore: %v", err)
	}

	t.Cleanup(func() {
		store.Close()
		SetMemoryStore(nil)
	})

	SetMemoryStore(store)

	t.Run("Easy", func(t *testing.T) { testEndToEndEasy(t, provider) })
	t.Run("Medium", func(t *testing.T) { testEndToEndMedium(t, provider) })
	t.Run("Hard", func(t *testing.T) { testEndToEndHard(t, provider) })
}

// newTestProvider returns a live provider for the suite, configured the same
// way src/provider.go's getProvider is (ollama's OpenAI-compatible endpoint),
// skipping the calling test rather than failing outright when ollama isn't
// reachable, since this suite needs infrastructure the rest of the package
// doesn't.
func newTestProvider(t *testing.T) anyllm.Provider {
	t.Helper()

	host := os.Getenv("OLLAMA_HOST")
	if host == "" {
		host = "http://localhost:11434"
	}
	baseURL := strings.TrimRight(host, "/") + "/v1"

	provider, err := openai.New(
		anyllm.WithBaseURL(baseURL),
		anyllm.WithAPIKey(e2eOllamaAPIKey),
		anyllm.WithTimeout(900*time.Second),
	)
	if err != nil {
		t.Fatalf("could not configure provider: %v", err)
	}

	if _, err := provider.ListModels(context.Background()); err != nil {
		t.Skipf("ollama not available, skipping end-to-end suite: %v", err)
	}

	if err := RefreshAgentContextWindows(host); err != nil {
		t.Logf("could not query model context windows: %v", err)
	}

	return provider
}

// runPlan drives prompt through the same sequence main.go's REPL loop
// runs — plan creation, objective execution, checklist verification, then
// summarization — and returns the final summary. It sets up its own
// perTestTimeout context rather than taking one from the caller, so every
// subtest's budget starts fresh at the beginning of its own runPlan call.
func runPlan(t *testing.T, provider anyllm.Provider, prompt string) string {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), perTestTimeout)
	defer cancel()

	plan, err := generatePlan(ctx, provider, prompt, "")
	if err != nil {
		if ctx.Err() != nil {
			t.Fatalf("generatePlan exceeded perTestTimeout (%s): %v", perTestTimeout, ctx.Err())
		}
		t.Fatalf("could not generatePlan: %v", err)
	}

	t.Logf("plan: %+v", plan)

	oc := ""

	result := chat(ctx, provider, plan.Agent, plan.Objective, oc, plan.PlanID, true)
	oc = appendContext(ctx, provider, oc, result)

	for i, check := range plan.Checklist {
		t.Logf("checking %d of %d: %s", i+1, len(plan.Checklist), check)

		checkResult := verifyCheck(ctx, provider, check, oc)

		oc = appendContext(ctx, provider, oc, checkResult)
	}

	summary := summarizePlanResults(ctx, provider, oc, plan.PlanID)

	if ctx.Err() != nil {
		t.Fatalf("runPlan exceeded perTestTimeout (%s): %v", perTestTimeout, ctx.Err())
	}

	return summary
}

// testEndToEndEasy is the easy case: a single, literal file write with no
// reasoning required.
func testEndToEndEasy(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "greeting.txt")

	prompt := fmt.Sprintf(
		`Create a file at %s containing exactly the text "Hello, Larkspur!" and nothing else.`,
		path,
	)

	summary := runPlan(t, provider, prompt)
	t.Logf("summary: %s", summary)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", path, err)
	}

	if got := strings.TrimSpace(string(data)); got != "Hello, Larkspur!" {
		t.Fatalf("Expected `Hello, Larkspur!`, received `%s`", got)
	}
}

// testEndToEndMedium is the medium case: it requires writing small, correct
// Go code rather than transcribing literal text. Verification parses the
// result with go/parser instead of judging the model's own account of what
// it wrote.
func testEndToEndMedium(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "add.go")

	prompt := fmt.Sprintf(
		"Write a Go source file at %s in package main that defines a function named Add "+
			"which takes two int parameters and returns their sum as an int.",
		path,
	)

	summary := runPlan(t, provider, prompt)
	t.Logf("summary: %s", summary)

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("generated file %s is not valid Go: %v", path, err)
	}

	var add *ast.FuncDecl

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "Add" {
			add = fn
			break
		}
	}

	if add == nil {
		t.Fatalf("Expected %s to define a function named Add", path)
	}

	if add.Type.Params.NumFields() != 2 {
		t.Fatalf("Expected Add to take 2 parameters, received %d", add.Type.Params.NumFields())
	}
}

// testEndToEndHard is the hard, multi-turn case: it requires reading one
// file, computing something from its content, and writing the result to a
// second file — a sequence of distinct tool calls a single ReAct turn can't
// collapse into one step, exercising the same continuity (checkpointing,
// compaction) the rest of this package's non-e2e tests can't reach.
func testEndToEndHard(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	dir := t.TempDir()
	logPath := filepath.Join(dir, "log.txt")
	countPath := filepath.Join(dir, "count.txt")

	logContent := "INFO: service started\n" +
		"ERROR: connection refused\n" +
		"INFO: retrying\n" +
		"ERROR: timeout\n" +
		"ERROR: connection refused\n" +
		"INFO: connected\n"

	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("could not WriteFile %s: %v", logPath, err)
	}

	prompt := fmt.Sprintf(
		`Read the file %s, count how many lines contain the word "ERROR", `+
			"and write just that number as plain text to %s.",
		logPath, countPath,
	)

	summary := runPlan(t, provider, prompt)
	t.Logf("summary: %s", summary)

	data, err := os.ReadFile(countPath)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", countPath, err)
	}

	if got := strings.TrimSpace(string(data)); got != "3" {
		t.Fatalf("Expected `3`, received `%s`", got)
	}
}
