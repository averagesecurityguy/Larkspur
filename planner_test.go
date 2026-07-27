package larkspur

import (
	"fmt"
	"testing"
)

type appendContextTest struct {
	context  string
	result   string
	expected string
}

func TestPlanner(t *testing.T) {
	t.Run("Testing AppendContext", testAppendContext)
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
