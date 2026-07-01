package larkspur

import (
	"context"

	"fmt"
	"log"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/mozilla-ai/any-llm-go/providers/ollama"
)

// Chat executes a ReAct loop using the given provider, model, and prompt.
// The final response is returned once the loop finishes.
func Chat(provider *ollama.Provider, model, prompt string, tools []anyllm.Tool) string {
	final := ""

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: developerPrompt},
		{Role: anyllm.RoleUser, Content: prompt},
	}

	for {
		ctx := context.Background()

		response, err := provider.Completion(ctx, anyllm.CompletionParams{
			Model:      model,
			Messages:   messages,
			Tools:      tools,
			ToolChoice: "auto",
		})
		if err != nil {
			log.Fatal(err)
		}

		message := response.Choices[0].Message
		finish := response.Choices[0].FinishReason
		final = fmt.Sprintf("%s", message.Content)

		fmt.Printf("Finish: %v\n", finish)
		fmt.Printf("Message: %s\n", final)

		if finish == anyllm.FinishReasonStop {
			break
		}

		if message.Reasoning != nil {
			fmt.Printf("Agent 🤔: %s\n", message.Reasoning.Content)
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
				fmt.Printf("  Tool: %s\n", tc.Function.Name)
				fmt.Printf("  Arguments: %s\n", tc.Function.Arguments)

				// Execute the real tool.
				result, execErr := executeTool(tc.Function.Name, tc.Function.Arguments)
				if execErr != nil {
					log.Fatal(execErr)
				}

				fmt.Printf("  Result: %s\n\n", result)

				// Add the tool result to the conversation.
				messages = append(messages, anyllm.Message{
					Role:       anyllm.RoleTool,
					Content:    result,
					ToolCallID: tc.ID,
				})
			}
		}
	}

	return final
}
