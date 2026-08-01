package larkspur

// planner defines the methods and data structures needed to create and verify
// an agentPlan. The agentPlan struct holds the plan while the planSchema
// defines the JSON schema for the agentPlan. The planSchema is used by the
// LLM to ensure a response is returned in the correct format. The planSchema
// and the agentPlan struct must stay in alignment.

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

const (
	maxAttempts = 5

	planPrompt = "Given the context, please create a plan to meet the following objective: %s\n"

	// maxOuterContextTokens is a hardcoded ceiling on the outer loop's
	// accumulated plan context (appendContext, below), independent of any
	// single agent's own contextTokens. Without it, a long-running plan
	// with many checklist items could grow that context so large that
	// every remaining chat() call has to immediately compact its incoming
	// promptContext (chat.go) just to get started — burning a
	// summarization call on every single turn instead of only
	// occasionally. Many current models cap out around 256K tokens, so
	// this is sized well under that on purpose: it's a safety valve, not a
	// target to fill.
	maxOuterContextTokens = 256000
)

// contextAppendThreshold is the character size at which the accumulated
// plan context is summarized by the compactor agent before the next result
// is appended. Sized off maxOuterContextTokens rather than any specific
// agent's own window, since the outer loop doesn't know in advance which
// agent will consume this context next (the objective agent, then the
// verifier, then the summarizer).
func contextAppendThreshold() int {
	return charBudget(maxOuterContextTokens)
}

// agentPlan holds a single plan that is used to accomplish one user goal. The
// agentPlan struct must stay in alignment with the schema in
// schemas/agent_plan.json. PlanID is not part of that schema — it is
// generated locally once the plan is parsed, and scopes the checkpoint that
// keeps the executing agent oriented across the objective and every
// checklist item.
type agentPlan struct {
	PlanID    string   `json:"-"`
	Objective string   `json:"objective"`
	Agent     string   `json:"agent"`
	Checklist []string `json:"checklist"`
}

// generatePlan creates and verifies a plan based on a user prompt. ctx
// bounds every completion call made along the way (see chat.go).
func generatePlan(ctx context.Context, provider anyllm.Provider, userPrompt, context string) (agentPlan, error) {
	var plan agentPlan

	userPrompt = fmt.Sprintf(planPrompt, userPrompt)
	planStr := chat(ctx, provider, planCreatorAgentName, userPrompt, context, "", true)

	// Trim any JSON markdown fencing
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

	plan.PlanID = rand.Text()

	return plan, nil
}

// planSummary holds the parsed output of the summarizer agent: a
// human-readable response plus any durable facts worth persisting to the
// memory store. planSummary must stay in alignment with the schema in
// schemas/plan_summary.json.
type planSummary struct {
	Response string         `json:"response"`
	Memories []memoryRecord `json:"memories"`
}

// memoryRecord is a single key/value memory identified by the summarizer as
// worth remembering across future sessions.
type memoryRecord struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// parsePlanSummary parses the summarizer agent's raw response into a
// planSummary, tolerating a response wrapped in a markdown JSON code fence.
func parsePlanSummary(raw string) (planSummary, error) {
	var summary planSummary

	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimSuffix(raw, "```")

	err := json.Unmarshal([]byte(raw), &summary)
	if err != nil {
		return summary, fmt.Errorf("could not parsePlanSummary: %v", err)
	}

	return summary, nil
}

// SummarizePlanResults produces a brief summary of the plan results,
// persists any memories the summarizer identified as worth remembering
// across future sessions, and clears planID's checkpoint now that the plan
// is finished.
func summarizePlanResults(ctx context.Context, provider anyllm.Provider, planResult, planID string) string {
	defer clearCheckpoint(planID)

	raw := chat(ctx, provider, planSummarizerAgentName, planResult, "", planID, false)

	summary, err := parsePlanSummary(raw)
	if err != nil {
		log.Error().Err(err).Msg("failed to parse plan summary")
		return raw
	}

	for _, record := range summary.Memories {
		if record.Key == "" {
			continue
		}

		if err := storeMemory(record.Key, record.Value); err != nil {
			log.Error().Err(err).Str("key", record.Key).Msg("could not store memory")
		}
	}

	return summary.Response
}

// VerifyCheck asks the dedicated verifier agent to confirm a single
// checklist item against promptContext, the accumulated record of completed
// work. It runs quietly (no per-tool-call output) and without a planID,
// since each checklist item is an independent, stateless check — unlike the
// objective execution it follows, it has no "next step" of its own for a
// checkpoint to carry forward.
func verifyCheck(ctx context.Context, provider anyllm.Provider, check, promptContext string) string {
	prompt := fmt.Sprintf("Verify the following has been completed: %s", check)
	return chat(ctx, provider, planVerifierAgentName, prompt, promptContext, "", false)
}

// appendContext adds a new result to the accumulated plan context,
// compacting the combined context with the compactor agent whenever it
// grows past contextAppendThreshold so a long-running plan with many
// checklist items doesn't grow the context handed to every remaining step
// without bound. This is a coarse, agent-agnostic ceiling — chat.go still
// does its own, tighter compaction of promptContext against whichever
// specific agent is about to consume it.
func appendContext(ctx context.Context, provider anyllm.Provider, context, result string) string {
	combined := result
	if context != "" {
		combined = fmt.Sprintf("%s\n%s", context, result)
	}

	if len(combined) <= contextAppendThreshold() {
		return combined
	}

	compacted := chat(ctx, provider, contextCompactorAgentName, combined, "", "", false)

	log.Debug().
		Int("before", len(combined)).
		Int("after", len(compacted)).
		Msg("compacted plan context")

	return compacted
}
