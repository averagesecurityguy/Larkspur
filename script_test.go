package larkspur

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestScript(t *testing.T) {
	t.Run("Testing runStarlark basics", testRunStarlarkBasics)
	t.Run("Testing runStarlark inputs", testRunStarlarkInputs)
	t.Run("Testing runStarlark print", testRunStarlarkPrint)
	t.Run("Testing runStarlark modules", testRunStarlarkModules)
	t.Run("Testing runStarlark errors", testRunStarlarkErrors)
	t.Run("Testing runStarlark has no file or process access", testRunStarlarkSandboxed)
	t.Run("Testing runStarlark step limit", testRunStarlarkStepLimit)
	t.Run("Testing runStarlark timeout", testRunStarlarkTimeout)
}

func testRunStarlarkBasics(t *testing.T) {
	fmt.Println(t.Name())

	response := runStarlark(`{"unexpected": "not a valid key"}`)
	if !strings.Contains(response, "run_starlark: error: missing script") {
		t.Fatalf("Expected a missing-script error, received `%s`", response)
	}

	response = runStarlark(`{"script": "result = 2 + 2"}`)
	if response != "result: 4" {
		t.Fatalf("Expected `result: 4`, received `%s`", response)
	}

	response = runStarlark(`{"script": "x = 1"}`)
	if !strings.Contains(response, "no output") {
		t.Fatalf("Expected a no-output message when result is unset, received `%s`", response)
	}

	response = runStarlark(`{"script": "result = sorted([3, 1, 2])"}`)
	if response != "result: [1,2,3]" {
		t.Fatalf("Expected `result: [1,2,3]`, received `%s`", response)
	}
}

func testRunStarlarkInputs(t *testing.T) {
	fmt.Println(t.Name())

	args := `{"script": "result = len(inputs[\"text\"].split(\"\\n\"))", "inputs": {"text": "a\nb\nc"}}`
	response := runStarlark(args)
	if response != "result: 3" {
		t.Fatalf("Expected `result: 3`, received `%s`", response)
	}

	args = `{"script": "result = [x * 2 for x in inputs[\"nums\"]]", "inputs": {"nums": [1, 2, 3]}}`
	response = runStarlark(args)
	if response != "result: [2,4,6]" {
		t.Fatalf("Expected `result: [2,4,6]`, received `%s`", response)
	}

	response = runStarlark(`{"script": "result = inputs"}`)
	if response != "result: {}" {
		t.Fatalf("Expected `result: {}` when inputs is omitted, received `%s`", response)
	}
}

func testRunStarlarkPrint(t *testing.T) {
	fmt.Println(t.Name())

	response := runStarlark(`{"script": "print(\"hello\")\nprint(\"world\")\nresult = 1"}`)
	if !strings.HasPrefix(response, "hello\nworld\n") {
		t.Fatalf("Expected printed lines before the result, received `%s`", response)
	}
	if !strings.HasSuffix(response, "result: 1") {
		t.Fatalf("Expected the result after the printed lines, received `%s`", response)
	}

	response = runStarlark(`{"script": "print(\"only printed, no result\")"}`)
	if response != "only printed, no result\n" {
		t.Fatalf("Expected just the printed output, received `%s`", response)
	}
}

func testRunStarlarkModules(t *testing.T) {
	fmt.Println(t.Name())

	response := runStarlark(`{"script": "result = math.sqrt(16)"}`)
	if response != "result: 4.0" {
		t.Fatalf("Expected `result: 4.0`, received `%s`", response)
	}

	response = runStarlark(`{"script": "result = json.decode(json.encode({\"a\": 1}))"}`)
	if response != `result: {"a":1}` {
		t.Fatalf("Expected `result: {\"a\":1}`, received `%s`", response)
	}
}

func testRunStarlarkErrors(t *testing.T) {
	fmt.Println(t.Name())

	response := runStarlark(`{"script": "this is not valid starlark ==="}`)
	if !strings.Contains(response, "run_starlark: error:") {
		t.Fatalf("Expected a syntax error, received `%s`", response)
	}

	response = runStarlark(`{"script": "result = 1 / 0"}`)
	if !strings.Contains(response, "run_starlark: error:") {
		t.Fatalf("Expected a runtime error, received `%s`", response)
	}

	// Output printed before a later failure is still returned, so partial
	// progress is visible for debugging.
	response = runStarlark(`{"script": "print(\"before the crash\")\nresult = 1 / 0"}`)
	if !strings.Contains(response, "before the crash") || !strings.Contains(response, "run_starlark: error:") {
		t.Fatalf("Expected printed output alongside the error, received `%s`", response)
	}
}

// testRunStarlarkSandboxed proves the predeclared environment has no
// filesystem or process access: names a Python/shell script might use for
// that are simply undefined in Starlark, so referencing them is a
// compile-time error, not a runtime capability.
func testRunStarlarkSandboxed(t *testing.T) {
	fmt.Println(t.Name())

	scripts := []string{
		`result = open("/etc/passwd")`,
		`result = os.system("echo hi")`,
		`result = exec("print(1)")`,
		`result = __import__("os")`,
	}

	for _, script := range scripts {
		args := fmt.Sprintf(`{"script": %q}`, script)
		response := runStarlark(args)
		if !strings.Contains(response, "run_starlark: error:") {
			t.Fatalf("Expected %q to fail as undefined, received `%s`", script, response)
		}
	}
}

func testRunStarlarkStepLimit(t *testing.T) {
	fmt.Println(t.Name())

	original := scriptMaxSteps
	scriptMaxSteps = 1000
	defer func() { scriptMaxSteps = original }()

	// Starlark's legacy file mode disallows a bare for loop at module
	// scope, so the loop is wrapped in a function — otherwise this would
	// fail as a syntax error before ever running, which would make the
	// test pass for the wrong reason.
	script := "def run():\n\tx = 0\n\tfor i in range(1000000):\n\t\tx += 1\n\treturn x\nresult = run()"
	response := runStarlark(fmt.Sprintf(`{"script": %q}`, script))
	if !strings.Contains(response, "run_starlark: error:") {
		t.Fatalf("Expected a step-limit error, received `%s`", response)
	}
	if !strings.Contains(response, "too many steps") && !strings.Contains(response, "cancelled") {
		t.Fatalf("Expected the error to mention the step limit, received `%s`", response)
	}
}

func testRunStarlarkTimeout(t *testing.T) {
	fmt.Println(t.Name())

	originalSteps := scriptMaxSteps
	originalTimeout := scriptTimeout
	scriptMaxSteps = 1_000_000_000_000
	scriptTimeout = 50 * time.Millisecond
	defer func() {
		scriptMaxSteps = originalSteps
		scriptTimeout = originalTimeout
	}()

	// Same reason as testRunStarlarkStepLimit: the loop must live inside a
	// function, or this fails as a syntax error rather than a timeout.
	script := "def run():\n\tx = 0\n\tfor i in range(1000000000):\n\t\tx += 1\n\treturn x\nresult = run()"

	start := time.Now()
	response := runStarlark(fmt.Sprintf(`{"script": %q}`, script))
	elapsed := time.Since(start)

	if !strings.Contains(response, "run_starlark: error:") {
		t.Fatalf("Expected a timeout error, received `%s`", response)
	}
	if !strings.Contains(response, "time limit") && !strings.Contains(response, "cancelled") {
		t.Fatalf("Expected the error to mention the time limit, received `%s`", response)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("Expected the timeout to cut execution short, took %s", elapsed)
	}
}
