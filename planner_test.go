package larkspur

import (
	"fmt"
	"slices"
	"testing"
)

type appendContextTest struct {
	context  string
	result   string
	expected string
}

type parsePlanSummaryTest struct {
	raw      string
	response string
	memories []memoryRecord
	wantErr  bool
}

func TestPlanner(t *testing.T) {
	t.Run("Testing AppendContext", testAppendContext)
	t.Run("Testing parsePlanSummary", testParsePlanSummary)
}

// testAppendContext only exercises the concatenation path, below
// contextAppendThreshold. The compaction path calls Chat with a live
// provider and is not covered here, matching the rest of the package: any
// path that reaches the provider is untested.
func testAppendContext(t *testing.T) {
	fmt.Println(t.Name())

	tests := []appendContextTest{
		{context: "", result: "first result", expected: "first result"},
		{context: "first result", result: "second result", expected: "first result\nsecond result"},
	}

	for _, test := range tests {
		got := AppendContext(nil, test.context, test.result)
		if got != test.expected {
			t.Fatalf("Expected `%s`, received `%s`", test.expected, got)
		}
	}
}

func testParsePlanSummary(t *testing.T) {
	fmt.Println(t.Name())

	tests := []parsePlanSummaryTest{
		{
			raw:      `{"response": "done", "memories": []}`,
			response: "done",
			memories: []memoryRecord{},
		},
		{
			raw:      "```json\n" + `{"response": "done", "memories": [{"key": "k", "value": "v"}]}` + "\n```",
			response: "done",
			memories: []memoryRecord{{Key: "k", Value: "v"}},
		},
		{
			raw:     `not json`,
			wantErr: true,
		},
	}

	for _, test := range tests {
		summary, err := parsePlanSummary(test.raw)

		if test.wantErr {
			if err == nil {
				t.Fatalf("Expected an error parsing `%s`", test.raw)
			}
			continue
		}

		if err != nil {
			t.Fatalf("could not parsePlanSummary: %v", err)
		}

		if summary.Response != test.response {
			t.Fatalf("Expected `%s`, received `%s`", test.response, summary.Response)
		}

		if !slices.Equal(summary.Memories, test.memories) {
			t.Fatalf("Expected `%v`, received `%v`", test.memories, summary.Memories)
		}
	}
}
