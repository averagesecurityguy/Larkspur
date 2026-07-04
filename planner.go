package larkspur

import (
	"fmt"
	"context"

	"github.com/rs/zerolog/log"
	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

var (
	plannerModel = "gemma4:e2b"
	plannerPrompt = `
	You analyze a user's request to understand their goal then you create a
	detailed list of step-by-step tasks that need to be completed in order to
	answer the user's request. You do not attempt to answer the user's request
	directly, you only return a PlannerResponse that contains the user's
	overall goal and a list of step-by-step tasks that will be completed by
	other agents. For each task, you need to decide the best agent to complete
	the task and you need to write a prompt for the agent that explains their
	task.

	# Using Tools
	There are a number of tools available for you to use when planning out the
	tasks. The prompt in a task should reference any appropriate tools along
	with their arguments.

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
	- **developer** - If the user's request requires any software development
	tasks such as writing programs, scripts, or functions or building software
	repository contents, route the request to the 'developer' agent.
	- **generalist** - If the user's request is not better served by one of the
	other agents, route it to the 'generalist' agent.

	# Creating Task Prompts
	The prompt in each task should be detailed enough for the named agent to
	complete its task but should not be overly verbose.
	`
	plannerSchemaStrict = true
	plannerResponse = &anyllm.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &anyllm.JSONSchema{
            Name:   "PlannerResponse",
            Strict: &plannerSchemaStrict,
            Schema: map[string]any{
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
            },
        },
    }
)

// Panner analyzes the user's prompt to determine the best agent and prompt to
// accomplish the goal.
func Planner(provider *ollama.Provider, userPrompt string) string {
	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: plannerPrompt},
		{Role: anyllm.RoleUser, Content: fmt.Sprintf("User's request: %s\n", userPrompt)},
	}

	ctx := context.Background()

	resp, err := provider.Completion(ctx, anyllm.CompletionParams{
		Model:      plannerModel,
		Messages:   messages,
		Tools:      loadAllTools(),
		ResponseFormat: plannerResponse,
		ReasoningEffort: anyllm.ReasoningEffortHigh,
	})
	if err != nil {
		log.Error().Err(err).Msg("invalid response")
		return "invalid response"
	}

	// if message.Reasoning != nil {
	// 	fmt.Printf("Agent 🤔: %s\n", message.Reasoning.Content)
	// 	messages = append(messages, anyllm.Message{
	// 		Role:    anyllm.RoleAssistant,
	// 		Content: message.Reasoning.Content,
	// 	})
	// }

	return fmt.Sprintf("%s", resp.Choices[0].Message.Content)
}