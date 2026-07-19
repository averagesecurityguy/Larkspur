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

// agentPlan holds a single plan that is used to accomplish one user goal. The
// agentPlan struct must stay in alignment with the schema in
// schemas/agent_plan.json.
type agentPlan struct {
	Objective string `json:"objective"`
	Goals     []goal `json:"goals"`
}

type goal struct {
	Goal    string `json:"goal"`
	Agent   string `json:"agent"`
	Prompt  string `json:"prompt"`
}

func (g goal) String() string {
	return fmt.Sprintf("%s 🤓: %s\n", g.Agent, g.Goal)
}

func (g *goal) Execute(p anyllm.Provider, con string) (string, error) {
	resp, err := Chat(p, g.Agent, g.Prompt, con)
	if err != nil {
		log.Error().Err(err).Msg("failed to execute task")
		return "", fmt.Errorf("failed to execute task")
	}

	return fmt.Sprintf("%s\n-----\n%s\n", g.Goal, resp), nil
}

// GeneratePlan creates and verifies a plan based on a user prompt.
func GeneratePlan(provider anyllm.Provider, userPrompt string) (agentPlan, error) {
	var plan agentPlan

	userPrompt = fmt.Sprintf("Please create a plan to meet the following objective: %s\n", userPrompt)

	planStr, err := Chat(provider, planCreatorAgentName, userPrompt, "")
	if err != nil {
		log.Error().Err(err).Msg("failed to generate a plan: creation")
		return plan, fmt.Errorf("failed to generate a plan: creation")
	}

	// planStr, err = Chat(provider, planVerifierAgentName, planStr, "")
	// if err != nil {
	// 	log.Error().Err(err).Msg("failed to generate a plan: verifier")
	// 	return plan, fmt.Errorf("failed to generate a plan: verifier")
	// }

	planStr = strings.TrimPrefix(planStr, "```json")
	planStr = strings.TrimSuffix(planStr, "```")

	log.Debug().
		Str("plan", planStr).
		Msg("final plan")

	err = json.Unmarshal([]byte(planStr), &plan)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate a plan: marshaler")
		return plan, fmt.Errorf("failed to gnerate a plan: marshaler")
	}

	return plan, nil
}

// SummarizePlanResults provides a brief summary of the plan results.
func SummarizePlanResults(provider anyllm.Provider, planResult string) string {
	result, err := Chat(provider, planSummarizerAgentName, planResult, "")
	if err != nil {
		log.Error().Err(err).Msg("failed to explain plan results")
		return ""
	}

	return result
}

