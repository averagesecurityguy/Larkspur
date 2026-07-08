package larkspur

// planner defines the methods and data structures needed to create and verify
// an agentPlan. The agentPlan struct holds the plan while the planSchema
// defines the JSON schema for the agentPlan. The planSchema is used by the
// LLM to ensure a response is returned in the correct format. The planSchema
// and the agentPlan struct must stay in alignment.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
	"github.com/rs/zerolog/log"
)

var (
	// Plan Creator
	agentPlanCreatorModel  = "gemma4:e2b"
	agentPlanCreatorTemp   = 0.9
	agentPlanCreatorTopP   = 0.95
	agentPlanCreatorPrompt = loadPrompt("prompts/plan_creator.md")

	// Plan Verifier
	agentPlanVerifierModel  = "gemma4:e2b"
	agentPlanVerifierTemp   = 0.9
	agentPlanVerifierTopP   = 0.95
	agentPlanVerifierPrompt = loadPrompt("prompts/plan_verifier.md")

	// Plan Schema
	schemaStrict      = true
	agentPlanResponse = &anyllm.ResponseFormat{
		Type:       "json_schema",
		JSONSchema: loadSchema("AgentPlanResponse", "schemas/agent_plan.json"),
	}
)

// loadPrompt loads the prompt at the given path or exits on failure.
func loadPrompt(path string) string {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not load prompt")
	}

	return string(data)
}

// loadSchema loads the schema at the given path or exits on failure.
func loadSchema(name, path string) *anyllm.JSONSchema {
	var as anyllm.JSONSchema

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not load schema")
	}

	schema := make(map[string]any)
	err = json.Unmarshal(data, &schema)
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not load schema")
	}

	as.Name = name
	as.Strict = &schemaStrict
	as.Schema = schema

	return &as
}

// agentPlan holds a single plan that is used to accomplish one user goal. The
// agentPlan struct must stay in alignment with the schema in
// schemas/agent_plan.json.
type agentPlan struct {
	UserGoal string      `json:"user_goal"`
	TaskList []agentTask `json:"task_list"`
}

// agentTask holds a single task needed to accomplish a user goal.
type agentTask struct {
	Actor  string `json:"actor"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

// planCreator analyzes the user's prompt to build an agentPlan that will
// accomplish the user's goal.
func planCreator(provider *ollama.Provider, userPrompt string) (agentPlan, error) {
	var plan agentPlan

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agentPlanCreatorPrompt},
		{Role: anyllm.RoleUser, Content: fmt.Sprintf("Analyze this user request: %s\n", userPrompt)},
	}

	ctx := context.Background()

	resp, err := provider.Completion(ctx, anyllm.CompletionParams{
		Model:           agentPlanCreatorModel,
		Messages:        messages,
		Tools:           loadAllTools(),
		Temperature:     &agentPlanCreatorTemp,
		TopP:            &agentPlanCreatorTopP,
		ResponseFormat:  agentPlanResponse,
		ReasoningEffort: anyllm.ReasoningEffortLow,
	})
	if err != nil {
		log.Error().Err(err).Msg("invalid response")
		return plan, err
	}

	planStr := fmt.Sprintf("%s", resp.Choices[0].Message.Content)

	// Convert the model response to an agentPlan
	err = json.Unmarshal([]byte(planStr), &plan)
	if err != nil {
		log.Error().Err(err).Str("plan", planStr).Msg("could not unmarshal response")
		log.Debug().Str("response", planStr)
		return plan, err
	}

	return plan, nil
}

// planVerifier analyzes the given plan to ensure the task list is sufficient
// to accomplish the goal.
func planVerifier(provider *ollama.Provider, plan agentPlan) (agentPlan, error) {
	planBytes, err := json.Marshal(plan)
	if err != nil {
		log.Error().Err(err).Msg("could not marshal plan")
		return plan, err
	}

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agentPlanVerifierPrompt},
		{Role: anyllm.RoleUser, Content: fmt.Sprintf("Review this AgentPlanResponse: %s", string(planBytes))},
	}

	ctx := context.Background()

	resp, err := provider.Completion(ctx, anyllm.CompletionParams{
		Model:           agentPlanVerifierModel,
		Messages:        messages,
		Tools:           loadAllTools(),
		ResponseFormat:  agentPlanResponse,
		ReasoningEffort: anyllm.ReasoningEffortHigh,
	})
	if err != nil {
		log.Error().Err(err).Msg("invalid response")
		return plan, err
	}

	planStr := fmt.Sprintf("%s", resp.Choices[0].Message.Content)
	fmt.Printf("PLAN STRING:\n%s\n\n", planStr)

	// Convert the model response to an agentPlan
	err = json.Unmarshal([]byte(planStr), &plan)
	if err != nil {
		log.Error().Err(err).Msg("could not parse response")
		return plan, err
	}

	return plan, nil
}

// GeneratePlan creates and verifies a plan based on a user prompt.
func GeneratePlan(provider *ollama.Provider, userPrompt string) (agentPlan, error) {
	plan, err := planCreator(provider, userPrompt)
	if err != nil {
		return plan, fmt.Errorf("failed to generate a plan: creation")
	}

	plan, err = planVerifier(provider, plan)
	if err != nil {
		return plan, fmt.Errorf("failed to generate a plan: verifier")
	}

	return plan, nil
}
