package larkspur

import (
	"context"
	"fmt"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

const (
	maxSnippet = 100

	// ollamaContextTokens mirrors the token budget the ollama provider
	// hardcodes for every request (any-llm-go's providers/ollama/ollama.go,
	// defaultNumCtx) regardless of what the underlying model supports.
	ollamaContextTokens = 32000

	// charsPerToken is a rough heuristic for English text and code (~4
	// characters per token), used to translate the token budget above into
	// the character counts we can actually measure without a real
	// tokenizer for the model. Every char-based threshold below is sized
	// off of this estimate rather than a bare number, so the relationship
	// to the real, token-based limit stays visible.
	charsPerToken = 4

	// compactThreshold is the total character size of the running message
	// list at which older history is summarized by the compactor agent.
	// That budget is shared across every try in the loop below, not just
	// the current turn. Triggering at roughly half the estimated character
	// budget leaves headroom for the tries and the final answer that
	// follow, without compacting so early that most of the available
	// context goes unused.
	compactThreshold = ollamaContextTokens * charsPerToken / 2
)

// Chat executes a ReAct loop using the given provider, model, and prompt.
// The final response is returned once the loop finishes.
func Chat(provider anyllm.Provider, agentName, prompt, promptContext string) string {
	var final string

	// Get the agent information from the agent name. This includes the model,
	// the tools, and the system prompt.
	agent := getAgent(agentName)

	// Set the initial messages for this chat. Includes the system prompt and
	// and the user's prompt.
	log.Debug().
		Str("model", agent.model).
		Str("system", agent.system).
		Float64("temp", agent.temp).
		Float64("topP", agent.topP).
		Msg("chat agent")

	log.Debug().
		Str("prompt", prompt).
		Msg("chat prompt")

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agent.system},
		{Role: anyllm.RoleUser, Content: promptContext},
		{Role: anyllm.RoleUser, Content: prompt},
	}

	// Run the ReAct loop appending each LLM response and the results of each
	// tool call to the message list.
	tries := 0
	for {
		ctx := context.Background()

		resp, err := provider.Completion(ctx, anyllm.CompletionParams{
			Model:           agent.model,
			Messages:        messages,
			Tools:           agent.tools,
			ToolChoice:      "auto",
			Temperature:     &agent.temp,
			TopP:            &agent.topP,
			ReasoningEffort: agent.reasoning,
		})
		if err != nil {
			log.Error().Err(err).Msg("invalid chat response")
			return fmt.Sprintf("chat: error: %v", err)
		}

		tries = tries + 1

		// Add the response message to the message list
		messages = append(messages, resp.Choices[0].Message)

		// Capture the message content in case it is our final message
		final = fmt.Sprintf("%s", resp.Choices[0].Message.Content)
		log.Debug().
			Str("response", final).
			Msg("chat response")

		// If the model wants to call tools, execute each one and append the
		// results to the message list.
		if resp.Choices[0].FinishReason == anyllm.FinishReasonToolCalls {
			for _, tc := range resp.Choices[0].Message.ToolCalls {
				result := executeTool(tc.Function.Name, tc.Function.Arguments)
				log.Debug().
					Str("function", tc.Function.Name).
					Str("arguments", tc.Function.Arguments).
					Msg(result)

				snippet := result
				if len(snippet) > maxSnippet {
					snippet = snippet[:maxSnippet]
				}
				fmt.Printf("[%s] %s\n%s\n", agentName, tc.Function.Name, snippet)

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

		// If we have reached our max attempts, break the loop and return the
		// final response.
		if tries >= agent.maxTries {
			break
		}

		// Keep the running conversation well under the provider's fixed
		// context window by summarizing older history once it grows past
		// the threshold, rather than letting it grow unbounded across
		// every remaining try.
		if messagesSize(messages) > compactThreshold {
			messages = compactHistory(provider, messages)
		}
	}

	return final
}

// contentStr renders a message's Content field, which is typed as any, as a
// plain string.
func contentStr(content any) string {
	if s, ok := content.(string); ok {
		return s
	}

	return fmt.Sprintf("%v", content)
}

// messagesSize returns the total character size of the content of the given
// messages, used as a cheap proxy for how much of the context window they
// occupy.
func messagesSize(messages []anyllm.Message) int {
	size := 0

	for _, m := range messages {
		size += len(contentStr(m.Content))
	}

	return size
}

// compactHistory replaces everything after the system prompt and the
// initial user messages with a single summary produced by the compactor
// agent. The system prompt and original prompt are always preserved
// untouched so the agent never loses its own instructions or task, only the
// accumulated tool calls and intermediate responses that led up to now.
func compactHistory(provider anyllm.Provider, messages []anyllm.Message) []anyllm.Message {
	// Nothing to compact yet: system prompt, context, and initial prompt.
	if len(messages) <= 3 {
		return messages
	}

	var history strings.Builder
	for _, m := range messages[3:] {
		fmt.Fprintf(&history, "%s: %s\n", m.Role, contentStr(m.Content))
	}

	summary := Chat(provider, contextCompactorAgentName, history.String(), "")

	log.Debug().
		Int("before", messagesSize(messages)).
		Str("summary", summary).
		Msg("compacted chat history")

	compacted := append([]anyllm.Message{}, messages[:3]...)
	compacted = append(compacted, anyllm.Message{
		Role:    anyllm.RoleUser,
		Content: fmt.Sprintf("Summary of work completed so far:\n%s", summary),
	})

	return compacted
}
