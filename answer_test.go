package larkspur

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// testAnswerPromptDirect exercises the router's "no plan needed" path: the
// router agent is the only agent called, and its answer is returned as-is.
func testAnswerPromptDirect(t *testing.T) {
	fmt.Println(t.Name())

	provider := &fakeProvider{
		responses: []string{`{"answer": "Paris", "need_plan": false}`},
	}

	answer, context := AnswerPrompt(context.Background(), provider, "what is the capital of France", "")

	if answer != "Paris" {
		t.Fatalf("Expected `Paris`, received `%s`", answer)
	}

	if context != "" {
		t.Fatalf("Expected context to be unchanged, received `%s`", context)
	}
}

// testAnswerPromptPlan exercises the full plan path: routing, plan
// creation, objective execution, checklist verification, and summarization,
// each of which calls a different agent in sequence.
func testAnswerPromptPlan(t *testing.T) {
	fmt.Println(t.Name())

	provider := &fakeProvider{
		responses: []string{
			// promptRouter: this request needs a plan.
			`{"answer": "", "need_plan": true}`,
			// generatePlan: a single-item checklist keeps this scripted
			// sequence short.
			`{"objective": "write a haiku", "agent": "generalist", "checklist": ["a haiku was written"]}`,
			// chat: executing the objective.
			"Here is a haiku about the sea.",
			// verifyCheck: the one checklist item.
			"Confirmed: a haiku was written.",
			// summarizePlanResults.
			`{"response": "Wrote a haiku about the sea.", "memories": []}`,
		},
	}

	answer, _ := AnswerPrompt(context.Background(), provider, "write me a haiku", "")

	if answer != "Wrote a haiku about the sea." {
		t.Fatalf("Expected `Wrote a haiku about the sea.`, received `%s`", answer)
	}
}

// testAnswerPromptPlanFailure exercises the error path when the plan
// creator's response can't be parsed as a plan: AnswerPrompt should report
// the failure rather than continue on to execute a zero-value plan.
func testAnswerPromptPlanFailure(t *testing.T) {
	fmt.Println(t.Name())

	provider := &fakeProvider{
		responses: []string{
			`{"answer": "", "need_plan": true}`,
			"not a plan",
		},
	}

	answer, context := AnswerPrompt(context.Background(), provider, "do something complicated", "")

	if !strings.Contains(answer, "unable to create a plan") {
		t.Fatalf("Expected a plan-failure message, received `%s`", answer)
	}

	if context != "" {
		t.Fatalf("Expected context to be unchanged, received `%s`", context)
	}
}

func TestAnswerPrompt(t *testing.T) {
	t.Run("Testing AnswerPrompt direct answer", testAnswerPromptDirect)
	t.Run("Testing AnswerPrompt full plan", testAnswerPromptPlan)
	t.Run("Testing AnswerPrompt plan failure", testAnswerPromptPlanFailure)
}
