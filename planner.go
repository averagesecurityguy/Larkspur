package larkspur

// planner defines the methods and data structures needed to create and verify
// an agentPlan. The agentPlan struct holds the plan while the planSchema
// defines the JSON schema for the agentPlan. The planSchema is used by the
// LLM to ensure a response is returned in the correct format. The planSchema
// and the agentPlan struct must stay in alignment.

import (
	"fmt"
	"context"
	"encoding/json"

	"github.com/rs/zerolog/log"
	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

var (
	agentPlanCreatorModel = "gemma4:e2b"
	agentPlanCreatorPrompt = `
	You analyze a user's request to understand their goal then you create a
	detailed list of step-by-step tasks that need to be completed by other LLM
	agents to achieve the user's goal. You do not attempt to answer the user's
	request directly, you only return an AgentPlanResponse that defines the
	user's overall goal and contains a list of step-by-step tasks that will be
	completed by other LLM agents. For each task, you need to decide the best
	LLM agent to complete the task and you need to write a prompt for the LLM
	agent that explains its task.

	# Using Tools
	There are a number of tools available to each of the LLM agents and these
	tools should be used when planning out the tasks. The task prompt should
	tell the LLM agent the appropriate tool and arguments to use to accomplish
	its task.

	# Example Request and Response
	If the user provides a request like, 'Summarize the contents of the
	agent.go file,' an appropriate response would look like:
	
	{
		"user_goal": "The user needs to summarize the contents of the file agent.go",
		"task_list": [
			{
				"agent": "generalist",
				"prompt": "Use the file_find_glob tool with the argument **/agent*.go to find the file and return its full path."
			},
			{
				"agent": "generalist",
				"prompt": "Use the read_file_full tool with the full path you previously identified to read the contents of the file."
			},
			{
				"agent": "generalist",
				"prompt": "Summarize the file contents you previously read and return the summary to the user."
			}
		]
	}

	# Available Agents
	- **developer** - If the user's goal requires any software development
	tasks such as writing programs, scripts, or functions or building software
	repository contents, route the request to the 'developer' agent.
	- **generalist** - If the user's request is not better served by one of the
	other agents, route it to the 'generalist' agent.

	# Creating Task Prompts
	When creating the prompt for each task ensure that it is detailed enough
	for the LLM agent to complete the task but do not make it overly verbose.
	The prompt should be written primarily for use by an LLM agent not a
	human user.
	`
	agentPlanVerifierModel = "gemma4:e2b"
	agentPlanVerifierPrompt = `
	You review an AgentPlanResponse to ensure the task_list is sufficient to
	meet the user_goal. You first review the task_list to ensure there are no
	missing tasks and that there are no tasks that need to be split up into
	smaller tasks. If there are missing tasks you create them and put them in
	the correct order within the task list. If a task needs to be split, you
	create the new tasks and replace the old task with the set of smaller
	tasks. Finally, you review each task to ensure the agent and prompt are
	correct and verify any recommended tools and arguments in the response.
	If there are no changes needed return the original plan as is. If changes
	are needed return the updated AgentPlanResponse.

	# Available Agents
	- **developer** - If the user's goal requires any software development
	tasks such as writing programs, scripts, or functions or building software
	repository contents, route the request to the 'developer' agent.
	- **generalist** - If the user's request is not better served by one of the
	other agents, route it to the 'generalist' agent.

	# Verifying Tools
	There are a number of tools available to each of the LLM agents and you
	should verify the correct tool and arguments were chosen to accomplish the
	given task.

	# Verifying Task Prompts
	When verifying the prompt for each task ensure that it is detailed enough
	for the LLM agent to complete the task, remembering the prompt should be
	written primarily for use by an LLM agent not a human user.
	`
	schemaStrict = true
	agentPlanResponse = &anyllm.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &anyllm.JSONSchema{
            Name:   "AgentPlanResponse",
            Strict: &schemaStrict,
            Schema: agentPlanSchema,
        },
    }
)

// agentPlan holds a single plan that is used to accomplish one user goal. The
// agentPlan must stay in alignment with the agentPlanSchema below.
type agentPlan struct {
	UserGoal string `json:"user_goal"`
	TaskList [] agentTask `json:"task_list"`
}

// agentTask holds a single task needed to accomplish a user goal.
type agentTask struct {
	Agent string `json:"agent"`
	Prompt string `json:"prompt"`
}

// agentPlanSchema defines the JSON schema used by the LLM to create it's
// response. The agentPlanSchema must stay in alignment with the agentPlan
// above.
var agentPlanSchema = map[string]any{
    "type": "object",
    "properties": map[string]any{
    	"user_goal": map[string]any{
    		"type": "string",
    		"description": "The overall goal the user is trying to achieve by completing these tasks.",
    	},
    	"task_list": map[string]any{
    		"type": "array",
    		"items": map[string]any{
    			"type": "object",
    			"properties": map[string]any{
    				"agent": map[string]any{
    					"type": "string",
    					"description": "the agent that should complete this task.",
    				},
    				"prompt": map[string]any{
    					"type": "string",
    					"description": "A prompt explaining to the agent the task that needs to be completed.",
    				},
    			},
    			"required": []string{"agent", "prompt"},
    		},
    	},
    },
    "required": []string{"user_goal", "task_list"},
}

// planCreator analyzes the user's prompt to build an agentPlan that will
// accomplish the user's goal.
func planCreator(provider *ollama.Provider, userPrompt string) (agentPlan, error) {
	var plan agentPlan

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agentPlanCreatorPrompt},
		{Role: anyllm.RoleUser, Content: fmt.Sprintf("User's request: %s\n", userPrompt)},
	}

	ctx := context.Background()

	resp, err := provider.Completion(ctx, anyllm.CompletionParams{
		Model:      agentPlanCreatorModel,
		Messages:   messages,
		Tools:      loadAllTools(),
		ResponseFormat: agentPlanResponse,
		ReasoningEffort: anyllm.ReasoningEffortHigh,
	})
	if err != nil {
		log.Error().Err(err).Msg("invalid response")
		return plan, err
	}

	planStr := fmt.Sprintf("%s", resp.Choices[0].Message.Content)

	// Convert the model response to an agentPlan
	err = json.Unmarshal([]byte(planStr), &plan)
	if err != nil {
		log.Error().Err(err).Msg("could not parse response")
		return plan, err
	}

	return plan, nil
}

// planVerifier analyzes the given plan to ensure the task list is sufficient
// to accomplish the goal.
func planVerifier(provider *ollama.Provider, plan agentPlan) (agentPlan, error) {
	planBytes, err := json.Marshal(plan)
	if err != nil {
		return plan, err
	}

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agentPlanVerifierPrompt},
		{Role: anyllm.RoleUser, Content: fmt.Sprintf("Analyze this AgentPlanResponse: %s", string(planBytes))},
	}

	ctx := context.Background()

	resp, err := provider.Completion(ctx, anyllm.CompletionParams{
		Model:      agentPlanVerifierModel,
		Messages:   messages,
		Tools:      loadAllTools(),
		ResponseFormat: agentPlanResponse,
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