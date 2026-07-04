package larkspur

import (
	"context"
	"fmt"
	"encoding/json"

	"github.com/rs/zerolog/log"
	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

type chatArguments struct {
	Agent string `json:"agent"`
	Prompt string `json:"prompt"`
}

// Route analyzes the user's prompt to determine the best agent and prompt to
// accomplish the goal.
func Route(provider *ollama.Provider, userPrompt string) string {
	final := ""

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: routerPrompt},
		{Role: anyllm.RoleUser, Content: fmt.Sprintf("User's request: %s\n", userPrompt)},
	}

	for {
		ctx := context.Background()

		response, err := provider.Completion(ctx, anyllm.CompletionParams{
			Model:      routerModel,
			Messages:   messages,
			ResponseFormat: routerResponse,
			ReasoningEffort: anyllm.ReasoningEffortHigh,
		})
		if err != nil {
			log.Error().Err(err).Msg("invalid response")
			break
		}

		message := response.Choices[0].Message
		finish := response.Choices[0].FinishReason
		final = fmt.Sprintf("%s", message.Content)

		if message.Reasoning != nil {
			fmt.Printf("Agent 🤔: %s\n", message.Reasoning.Content)
			messages = append(messages, anyllm.Message{
				Role:    anyllm.RoleAssistant,
				Content: message.Reasoning.Content,
			})
		}

		if finish == anyllm.FinishReasonStop {
			break
		}
	}

	return final
}


// Chat executes a ReAct loop using the given provider, model, and prompt.
// The final response is returned once the loop finishes.
func Chat(provider *ollama.Provider, route string) string {
	var final string
	var args chatArguments

	err := json.Unmarshal([]byte(route), &args)
	if err != nil {
		log.Error().Err(err).Msg("invalid arguments")
		return ""
	}

	agent := getAgent(args.Agent)

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agent.system},
		{Role: anyllm.RoleUser, Content: args.Prompt},
	}

	for {
		ctx := context.Background()

		response, err := provider.Completion(ctx, anyllm.CompletionParams{
			Model:      agent.model,
			Messages:   messages,
			Tools:      agent.tools,
			ToolChoice: "auto",
		})
		if err != nil {
			log.Error().Err(err).Msg("invalid response")
			break
		}

		message := response.Choices[0].Message
		finish := response.Choices[0].FinishReason
		final = fmt.Sprintf("%s", message.Content)

		if message.Reasoning != nil {
			fmt.Printf("%s 🤔: %s\n", args.Agent, message.Reasoning.Content)
			messages = append(messages, anyllm.Message{
				Role:    anyllm.RoleAssistant,
				Content: message.Reasoning.Content,
			})
		}

		// Check if the model wants to call a tool.
		if finish == anyllm.FinishReasonToolCalls {
			// Add the assistant's message (with tool calls) to the conversation.
			messages = append(messages, message)

			// Process each tool call.
			for _, tc := range response.Choices[0].Message.ToolCalls {
				// Execute the real tool.
				result := executeTool(tc.Function.Name, tc.Function.Arguments)

				// Add the tool result to the conversation.
				messages = append(messages, anyllm.Message{
					Role:       anyllm.RoleTool,
					Content:    result,
					ToolCallID: tc.ID,
				})
			}
		}

		if finish == anyllm.FinishReasonStop {
			break
		}
	}

	return final
}
