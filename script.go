package larkspur

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"go.starlark.net/starlark"

	starlarkjson "go.starlark.net/lib/json"
	starlarkmath "go.starlark.net/lib/math"
)

// scriptMaxSteps bounds a run_starlark call to this many abstract
// computation steps — go.starlark.net's own cost unit, incremented once
// per bytecode instruction, not wall-clock time — so a runaway loop can't
// consume unbounded CPU. scriptTimeout is a wall-clock backstop for the
// same purpose, covering the rarer case where few steps still take a
// long time (e.g. one very large string or list operation). Both are vars,
// not consts, so tests can shrink them to exercise cancellation quickly.
var (
	scriptMaxSteps uint64 = 100_000_000
	scriptTimeout         = 10 * time.Second
)

type runStarlarkArgs struct {
	Script string          `json:"script"`
	Inputs json.RawMessage `json:"inputs"`
}

// runStarlark executes a Starlark script — a deterministic, Python-like
// language with no built-in access to the filesystem, network, or process
// execution (see https://github.com/google/starlark-go) — as a safe
// substitute for the arbitrary code execution a model might otherwise
// reach for (python -c, a throwaway script run via system_command). It
// cannot read or write anything on its own: any data the script needs
// must be supplied via the inputs argument, read first through the file
// tools, and is exposed to the script as a predeclared global named
// inputs.
//
// If the script sets a global variable named result, that value is
// JSON-encoded (via Starlark's own json.encode) and returned; any print()
// calls made during execution are also captured and included ahead of it.
// json and math modules are predeclared for the script to use directly.
func runStarlark(arguments string) string {
	var args runStarlarkArgs

	if err := json.Unmarshal([]byte(arguments), &args); err != nil {
		log.Error().Err(err).Msg("could not parse arguments")
		return fmt.Sprintf("run_starlark: error: %v", err)
	}

	if args.Script == "" {
		log.Error().Msg("missing script")
		return "run_starlark: error: missing script"
	}

	inputsJSON := args.Inputs
	if len(inputsJSON) == 0 {
		inputsJSON = json.RawMessage("{}")
	}

	var printed strings.Builder

	thread := &starlark.Thread{
		Name: "run_starlark",
		Print: func(_ *starlark.Thread, msg string) {
			printed.WriteString(msg)
			printed.WriteString("\n")
		},
	}
	thread.SetMaxExecutionSteps(scriptMaxSteps)

	timer := time.AfterFunc(scriptTimeout, func() {
		thread.Cancel("exceeded run_starlark's time limit")
	})
	defer timer.Stop()

	inputs, err := decodeToStarlark(thread, string(inputsJSON))
	if err != nil {
		log.Error().Err(err).Msg("could not decode inputs")
		return fmt.Sprintf("run_starlark: error: could not parse inputs: %v", err)
	}

	predeclared := starlark.StringDict{
		"json":   starlarkjson.Module,
		"math":   starlarkmath.Module,
		"inputs": inputs,
	}

	globals, err := starlark.ExecFile(thread, "run_starlark.star", args.Script, predeclared)

	var response strings.Builder
	if printed.Len() > 0 {
		response.WriteString(printed.String())
	}

	if err != nil {
		log.Error().Err(err).Msg("run_starlark script failed")

		if response.Len() > 0 {
			response.WriteString("\n")
		}
		fmt.Fprintf(&response, "run_starlark: error: %v", err)

		return response.String()
	}

	result, ok := globals["result"]
	if !ok {
		if response.Len() == 0 {
			return "run_starlark: script ran successfully but produced no output " +
				"(set a `result` variable or call print() to return something)"
		}

		return response.String()
	}

	encoded, err := encodeFromStarlark(thread, result)
	if err != nil {
		log.Error().Err(err).Msg("could not encode result")

		if response.Len() > 0 {
			response.WriteString("\n")
		}
		fmt.Fprintf(&response, "run_starlark: error: could not encode result: %v", err)

		return response.String()
	}

	if response.Len() > 0 {
		response.WriteString("\n")
	}
	fmt.Fprintf(&response, "result: %s", encoded)

	return response.String()
}

// decodeToStarlark parses jsonText into a Starlark value using
// go.starlark.net's own json.decode, so nested objects, arrays, numbers,
// strings, booleans, and null convert exactly the way a Starlark script
// calling json.decode itself would expect.
func decodeToStarlark(thread *starlark.Thread, jsonText string) (starlark.Value, error) {
	decode, ok := starlarkjson.Module.Members["decode"]
	if !ok {
		return nil, fmt.Errorf("json.decode not available")
	}

	return starlark.Call(thread, decode, starlark.Tuple{starlark.String(jsonText)}, nil)
}

// encodeFromStarlark renders a Starlark value as a JSON string using
// go.starlark.net's own json.encode.
func encodeFromStarlark(thread *starlark.Thread, value starlark.Value) (string, error) {
	encode, ok := starlarkjson.Module.Members["encode"]
	if !ok {
		return "", fmt.Errorf("json.encode not available")
	}

	encoded, err := starlark.Call(thread, encode, starlark.Tuple{value}, nil)
	if err != nil {
		return "", err
	}

	str, ok := encoded.(starlark.String)
	if !ok {
		return "", fmt.Errorf("json.encode returned %s, not a string", encoded.Type())
	}

	return string(str), nil
}
