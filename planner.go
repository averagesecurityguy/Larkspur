package larkspur

// planner defines the methods and data structures needed to create and verify
// an agentPlan. The agentPlan struct holds the plan while the planSchema
// defines the JSON schema for the agentPlan. The planSchema is used by the
// LLM to ensure a response is returned in the correct format. The planSchema
// and the agentPlan struct must stay in alignment.

import (
	"encoding/json"
	"fmt"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

const (
	maxAttempts = 5

	// contextAppendThreshold is the character size at which the accumulated
	// plan context is summarized by the compactor agent before the next
	// result is appended. It's kept well below compactThreshold (chat.go)
	// on purpose: this accumulated context becomes the seed for a fresh
	// Chat call's message history, so if it were allowed to grow as large
	// as compactThreshold by itself, that call's own ReAct loop would have
	// no headroom left before needing to compact again.
	contextAppendThreshold = compactThreshold / 2
)

// agentPlan holds a single plan that is used to accomplish one user goal. The
// agentPlan struct must stay in alignment with the schema in
// schemas/agent_plan.json.
type agentPlan struct {
	Objective string   `json:"objective"`
	Agent     string   `json:"agent"`
	Checklist []string `json:"checklist"`
}

// GeneratePlan creates and verifies a plan based on a user prompt.
func GeneratePlan(provider anyllm.Provider, userPrompt string) (agentPlan, error) {
	var plan agentPlan

	userPrompt = fmt.Sprintf("Please create a plan to meet the following objective: %s\n", userPrompt)

	planStr := Chat(provider, planCreatorAgentName, userPrompt, "")

	planStr = strings.TrimPrefix(planStr, "```json")
	planStr = strings.TrimSuffix(planStr, "```")

	log.Debug().
		Str("plan", planStr).
		Msg("final plan")

	err := json.Unmarshal([]byte(planStr), &plan)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate a plan: marshaler")
		return plan, fmt.Errorf("failed to gnerate a plan: marshaler")
	}

	return plan, nil
}

// SummarizePlanResults provides a brief summary of the plan results.
func SummarizePlanResults(provider anyllm.Provider, planResult string) string {
	return Chat(provider, planSummarizerAgentName, planResult, "")
}

// AppendContext adds a new result to the accumulated plan context,
// compacting the combined context with the compactor agent whenever it
// grows past contextAppendThreshold so a plan with many checklist items
// doesn't grow the context handed to every remaining step without bound.
func AppendContext(provider anyllm.Provider, context, result string) string {
	combined := result
	if context != "" {
		combined = fmt.Sprintf("%s\n%s", context, result)
	}

	if len(combined) <= contextAppendThreshold {
		return combined
	}

	compacted := Chat(provider, contextCompactorAgentName, combined, "")

	log.Debug().
		Int("before", len(combined)).
		Int("after", len(compacted)).
		Msg("compacted plan context")

	return compacted
}
