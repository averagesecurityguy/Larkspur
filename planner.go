package larkspur

// planner defines the methods and data structures needed to create and verify
// an agentPlan. The agentPlan struct holds the plan while the planSchema
// defines the JSON schema for the agentPlan. The planSchema is used by the
// LLM to ensure a response is returned in the correct format. The planSchema
// and the agentPlan struct must stay in alignment.

import (
	"crypto/rand"
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

// GeneratePlan creates and verifies a plan based on a user prompt.
func GeneratePlan(provider anyllm.Provider, userPrompt string) (agentPlan, error) {
	var plan agentPlan

	userPrompt = fmt.Sprintf("Please create a plan to meet the following objective: %s\n", userPrompt)

	planStr := Chat(provider, planCreatorAgentName, userPrompt, "", "", true)

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
func SummarizePlanResults(provider anyllm.Provider, planResult, planID string) string {
	defer clearCheckpoint(planID)

	raw := Chat(provider, planSummarizerAgentName, planResult, "", planID, false)

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

	compacted := Chat(provider, contextCompactorAgentName, combined, "", "", false)

	log.Debug().
		Int("before", len(combined)).
		Int("after", len(compacted)).
		Msg("compacted plan context")

	return compacted
}
