//go:build e2e

package larkspur

// e2e_test defines a small suite of prompts that exercise larkspur's full
// pipeline (plan creation, objective execution, checklist verification, and
// summarization) against a live ollama provider. Unlike the rest of this
// package's tests, these hit the provider and a real model on purpose, so
// they're gated behind the "e2e" build tag and excluded from `go test
// ./...`. Run them explicitly:
//
//	go test -tags e2e -timeout 130m -run TestEndToEnd -v .
//
// ollama must be running locally with the model configured in agents.go
// (generalist, built from modelfiles/generalist.Modelfile — see the
// README's Ollama setup section) available. If it isn't reachable, the
// suite skips instead of failing. Assertions check observable
// filesystem state (what the agent's tools actually produced), not the
// model's prose, since that varies run to run. Even so, a small local
// model can occasionally fail a task outright,
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
	"crypto/rand"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
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
	t.Run("Memory", func(t *testing.T) { testEndToEndMemory(t, provider) })
	t.Run("SystemCommand", func(t *testing.T) { testEndToEndSystemCommand(t, provider) })
	t.Run("Search", func(t *testing.T) { testEndToEndSearch(t, provider) })
	t.Run("Edit", func(t *testing.T) { testEndToEndEdit(t, provider) })
	t.Run("CheckpointRecovery", func(t *testing.T) { testEndToEndCheckpointRecovery(t, provider) })
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

// testEndToEndMemory proves memories persist across independent plans
// sharing the same store: one plan writes a value via memory_put, and a
// second, unrelated plan later recalls it via memory_get and acts on
// it — the cross-session continuity the memory_* tools and
// SummarizePlanResults exist to provide, exercised here end-to-end for
// the first time. Verification of the first half reads the store
// directly (ground truth), rather than trusting the model's own account
// of what it stored.
func testEndToEndMemory(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	const key = "e2e_test_codename"
	const value = "NERINE-7"

	putPrompt := fmt.Sprintf(
		`Store the value "%s" in memory under the key "%s" so it can be recalled later.`,
		value, key,
	)

	summary := runPlan(t, provider, putPrompt)
	t.Logf("summary: %s", summary)

	stored, err := memoryStore.Get(key)
	if err != nil {
		t.Fatalf("could not Get %s from memory store: %v", key, err)
	}

	if stored != value {
		t.Fatalf("Expected memory %s to be %q, received %q", key, value, stored)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "codename.txt")

	getPrompt := fmt.Sprintf(
		`Recall the memory stored under the key "%s" and write its value to the file %s.`,
		key, outPath,
	)

	summary = runPlan(t, provider, getPrompt)
	t.Logf("summary: %s", summary)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", outPath, err)
	}

	if got := strings.TrimSpace(string(data)); got != value {
		t.Fatalf("Expected `%s`, received `%s`", value, got)
	}
}

// testEndToEndSystemCommand requires chaining two distinct tools with a
// real dependency between them: write a Go file, then run an
// allow-listed command (gofmt -l) against it via system_command and
// report what that command actually found. Verification runs gofmt -l
// itself to get ground truth rather than trusting either the model's
// code or its own report of whether it was clean.
func testEndToEndSystemCommand(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "double.go")
	resultPath := filepath.Join(dir, "fmt_result.txt")

	prompt := fmt.Sprintf(
		"Write a Go source file at %s in package main that defines a function named Double "+
			"which takes one int parameter and returns double its value. Then check whether "+
			"that file is gofmt-clean by running \"gofmt -l %s\", and write the word CLEAN "+
			"to %s if that produces no output, or DIRTY to %s if it produces any output.",
		path, path, resultPath, resultPath,
	)

	summary := runPlan(t, provider, prompt)
	t.Logf("summary: %s", summary)

	fset := token.NewFileSet()

	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("generated file %s is not valid Go: %v", path, err)
	}

	var double *ast.FuncDecl

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == "Double" {
			double = fn
			break
		}
	}

	if double == nil {
		t.Fatalf("Expected %s to define a function named Double", path)
	}

	out, err := exec.Command("gofmt", "-l", path).CombinedOutput()
	if err != nil {
		t.Fatalf("could not run gofmt -l %s: %v", path, err)
	}

	want := "CLEAN"
	if strings.TrimSpace(string(out)) != "" {
		want = "DIRTY"
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", resultPath, err)
	}

	if got := strings.TrimSpace(string(data)); got != want {
		t.Fatalf("Expected %q (per gofmt -l), received %q", want, got)
	}
}

// testEndToEndSearch requires finding a needle across several decoy
// files rather than reading a file it was already handed the exact path
// to. The prompt deliberately points at a nonexistent file first, so the
// agent must recover from a failed read and use dir_list/file_find_glob/
// grep_files to actually locate the answer instead of stalling or
// fabricating one.
func testEndToEndSearch(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	dir := t.TempDir()

	files := map[string]string{
		"alpha.log":   "INFO: startup complete\nINFO: listening on :8080\n",
		"bravo.log":   "INFO: connected to db\nAUTH-TOKEN: XJ4Q9Z\nINFO: ready\n",
		"charlie.log": "WARN: retrying connection\nINFO: connected\n",
	}

	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatalf("could not WriteFile %s: %v", name, err)
		}
	}

	outPath := filepath.Join(dir, "answer.txt")

	prompt := fmt.Sprintf(
		`Start by trying to read the file %s — it does not exist, so that read will fail. `+
			`When it does, search the directory %s for the .log file that contains a line `+
			`starting with "AUTH-TOKEN:". Write just that file's name (e.g. "alpha.log", not `+
			"the full path) to %s.",
		filepath.Join(dir, "config.log"), dir, outPath,
	)

	summary := runPlan(t, provider, prompt)
	t.Logf("summary: %s", summary)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", outPath, err)
	}

	if got := strings.TrimSpace(string(data)); got != "bravo.log" {
		t.Fatalf("Expected `bravo.log`, received `%s`", got)
	}
}

// testEndToEndEdit requires modifying one line of an existing file via
// file_edit rather than rewriting the whole thing. Checking that the
// untouched line survives is what actually distinguishes an in-place
// edit from a full file_write_full rewrite that happened to reproduce
// the same content.
func testEndToEndEdit(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.txt")

	if err := os.WriteFile(path, []byte("debug=false\ntimeout=30\n"), 0644); err != nil {
		t.Fatalf("could not WriteFile %s: %v", path, err)
	}

	prompt := fmt.Sprintf(
		`The file %s contains a line "debug=false". Change just that line to "debug=true", `+
			`leaving the rest of the file untouched.`,
		path,
	)

	summary := runPlan(t, provider, prompt)
	t.Logf("summary: %s", summary)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", path, err)
	}

	got := string(data)

	if !strings.Contains(got, "debug=true") {
		t.Fatalf("Expected %s to contain `debug=true`, received `%s`", path, got)
	}

	if strings.Contains(got, "debug=false") {
		t.Fatalf("Expected %s to no longer contain `debug=false`, received `%s`", path, got)
	}

	if !strings.Contains(got, "timeout=30") {
		t.Fatalf(
			"Expected %s to still contain `timeout=30` (an in-place edit, not a full rewrite), received `%s`",
			path, got,
		)
	}
}

// testEndToEndCheckpointRecovery proves the checkpoint mechanism itself
// (memory.go's getCheckpoint/putCheckpoint, wired into chat.go) actually
// carries information across two independent chat() calls sharing a
// planID — the same continuity a plan relies on across its checklist
// items or across a history compaction. It drives chat() directly rather
// than the full plan pipeline, since what's under test is continuity
// between two separate calls, not plan creation or checklist
// verification.
func testEndToEndCheckpointRecovery(t *testing.T, provider anyllm.Provider) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), perTestTimeout)
	defer cancel()

	planID := rand.Text()
	t.Cleanup(func() { clearCheckpoint(planID) })

	const breadcrumb = "resume: write PINEAPPLE to the output file"

	setupPrompt := fmt.Sprintf(
		"Call the task_checkpoint tool with next_step set to exactly this text, "+
			"and do nothing else: %s",
		breadcrumb,
	)

	chat(ctx, provider, generalistAgentName, setupPrompt, "", planID, true)

	checkpoint := getCheckpoint(planID)
	if !strings.Contains(checkpoint, "PINEAPPLE") {
		t.Fatalf("Expected checkpoint for %s to contain `PINEAPPLE`, received `%s`", planID, checkpoint)
	}

	dir := t.TempDir()
	outPath := filepath.Join(dir, "output.txt")

	resumePrompt := fmt.Sprintf(
		"Your checkpoint reminder tells you what to do next. Follow it literally, using %s "+
			"as the output file it refers to.",
		outPath,
	)

	chat(ctx, provider, generalistAgentName, resumePrompt, "", planID, true)

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("could not ReadFile %s: %v", outPath, err)
	}

	if got := strings.TrimSpace(string(data)); got != "PINEAPPLE" {
		t.Fatalf("Expected `PINEAPPLE`, received `%s`", got)
	}
}
