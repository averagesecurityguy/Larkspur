package larkspur

import (
	"context"
	"fmt"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

const (
	noPlan = "Lark: I was unable to create a plan: %v./n"
)

// AnswerPrompt returns the answer to the user's prompt along with any
// accumulated context from answering the prompt. ctx bounds every
// completion call made along the way (see chat.go); production callers
// with no need for a deadline can pass context.Background().
func AnswerPrompt(ctx context.Context, provider anyllm.Provider, prompt, context string) (string, string) {
	// Determine if the prompt is simple enough to be answered without a plan.
	// If it is, return the answer directly.
	fmt.Println("Lark: Thinking 🤔...")
	answer, needPlan := promptRouter(ctx, provider, prompt, context)
	if !needPlan {
		return answer, context
	}

	// The router has determined we need a plan so let's build one.
	fmt.Println("Lark: Planning 🤓...")
	plan, err := generatePlan(ctx, provider, prompt, context)
	if err != nil {
		return fmt.Sprintf(noPlan, err), context
	}

	// Work through the plan
	fmt.Println("Lark: Working 🫡...")
	result := chat(ctx, provider, plan.Agent, plan.Objective, context, plan.PlanID, true)
	context = appendContext(ctx, provider, context, result)

	// Verify the plan one check at a time
	fmt.Println("Lark: Verifying 🧐...")
	for i, check := range plan.Checklist {
		fmt.Printf("Checking %d of %d: %s ... ", i+1, len(plan.Checklist), check)

		result := verifyCheck(ctx, provider, check, context)
		fmt.Printf("✓\n")

		context = appendContext(ctx, provider, context, result)
	}

	answer = summarizePlanResults(ctx, provider, context, plan.PlanID)

	return answer, context
}
