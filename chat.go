package larkspur

import (
	"context"
	"fmt"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
	"github.com/rs/zerolog/log"
)

// Chat executes a ReAct loop using the given provider, model, and prompt.
// The final response is returned once the loop finishes.
func Chat(provider *ollama.Provider, agentName, prompt string) (string, error) {
	var final string

	// Get the agent information from the agent name. This includes the model,
	// the tools, and the system prompt.
	agent := getAgent(agentName)

	// Set the initial messages for this chat. Includes the system prompt and
	// and the user's prompt.
	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agent.system},
		{Role: anyllm.RoleUser, Content: prompt},
	}

	// Run the ReAct loop appending each LLM response and the results of each
	// tool call to the message list.
	for {
		ctx := context.Background()

		resp, err := provider.Completion(ctx, anyllm.CompletionParams{
			Model:      agent.model,
			Messages:   messages,
			Tools:      agent.tools,
			ToolChoice: "auto",
		})
		if err != nil {
			log.Error().Err(err).Msg("invalid response")
			return "", err
		}

		log.Debug().Msg(fmt.Sprintf("%v", resp))

		// Add the response message to the message list
		messages = append(messages, resp.Choices[0].Message)

		// Capture the message content in case it is our final message
		final = fmt.Sprintf("%s", resp.Choices[0].Message.Content)

		// If the model wants to call tools, execute each one and append the
		// results to the message list.
		if resp.Choices[0].FinishReason == anyllm.FinishReasonToolCalls {
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				result := ExecuteTool(tc.Function.Name, tc.Function.Arguments)
				log.Debug().
					Str("function", tc.Function.Name).
					Str("arguments", tc.Function.Arguments).
					Msg(result)

				// Add the tool result to the conversation.
				messages = append(messages, anyllm.Message{
					Role:       anyllm.RoleTool,
					Content:    result,
					ToolCallID: tc.ID,
				})
			}
		}

		// If the model is done then break the loop and return the final response.
		if resp.Choices[0].FinishReason == anyllm.FinishReasonStop {
			break
		}
	}

	return final, nil
}
