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
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
	"github.com/rs/zerolog/log"
)

type Tasker interface {
	Execute(anyllm.Provider, string) string
}

// agentPlan holds a single plan that is used to accomplish one user goal. The
// agentPlan struct must stay in alignment with the schema in
// schemas/agent_plan.json.
type agentPlan struct {
	Objective string `json:"objective"`
	Goals     []goal `json:"goals"`
}

type goal struct {
	Goal     string   `json:"goal"`
	TaskList []Tasker `json:"task_list"`
}

// agentTask holds a single task needed to accomplish a user goal.
type agentTask struct {
	Agent  string `json:"agent"`
	Prompt string `json:"prompt"`
}

func (at agentTask) String() string {
	return fmt.Sprintf("%s 🤓: %s\n", at.Agent, at.Prompt)
}

func (at *agentTask) Execute(p *ollama.Provider, con string) string {
	resp, err := Chat(p, at.Agent, at.Prompt, con)
	if err != nil {
		fmt.Println("Agent ☹️: I was unable to execute the task.")
		return ""
	}

	return fmt.Sprintf("%s\n-----\n%s\n", at.Prompt, resp)
}

// toolTask holds a single task needed to accomplish a user goal.
type toolTask struct {
	Function string `json:"function"`
	Params   string `json:"params"`
}

func (tt toolTask) String() string {
	return fmt.Sprintf("%s ⚙️: %s", tt.Function, tt.Params)
}

func (tt *toolTask) Execute(p *ollama.Provider, con string) string {
	result := executeTool(tt.Function, tt.Params)

	return fmt.Sprintf("%s\n-----\n%s\n", tt.Function, result)
}

// GeneratePlan creates and verifies a plan based on a user prompt.
func GeneratePlan(provider *ollama.Provider, userPrompt string) (agentPlan, error) {
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
