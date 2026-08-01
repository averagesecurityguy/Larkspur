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

	// charsPerToken is a rough heuristic for English text and code (~4
	// characters per token), used to translate a token budget into the
	// character counts we can actually measure without a real tokenizer
	// for the model. Every char-based threshold below is sized off of
	// this estimate rather than a bare number, so the relationship to the
	// real, token-based limit stays visible.
	charsPerToken = 4
)

// charBudget converts a token budget into the character threshold at which
// accumulated content should be considered for compaction: charsPerToken
// translates tokens to characters, and triggering at roughly half of that
// leaves headroom for what follows rather than compacting right at the
// edge.
func charBudget(tokens int) int {
	return tokens * charsPerToken / 2
}

// compactThreshold is the total character size of the running message list,
// in a single chat() call, at which older history is summarized by the
// compactor agent. Sized off this agent's own contextTokens rather than any
// other agent's, since a chat() call only ever talks to this one model — an
// agent with a larger window gets to use all of it, not whatever the
// smallest-context agent in the system happens to support.
func (a *agent) compactThreshold() int {
	return charBudget(a.contextTokens)
}

// Chat executes a ReAct loop using the given provider, model, and prompt.
// ctx bounds every completion call this makes, including the recursive
// calls it makes into itself for compaction — callers that need a hard
// ceiling on how long a single chat() call can run (e.g. a test against a
// slow local model) should pass a context with a deadline; production
// callers that don't need one can pass context.Background(). planID scopes
// the run to a plan's checkpoint, letting the agent remind itself what to
// do next across a history compaction and across the separate Chat calls
// that make up a single plan (the objective, then each checklist item).
// Pass "" when no plan is in progress, e.g. plan creation itself or the
// compactor's own summarization calls. verbose controls whether each tool
// call is printed as it happens; callers that only need the final
// response, such as checklist verification, should pass false to keep that
// detail out of the user's view. The final response is returned once the
// loop finishes.
func chat(ctx context.Context, provider anyllm.Provider, agentName, prompt, promptContext, planID string, verbose bool) string {
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

	// promptContext is the outer loop's accumulated plan context
	// (planner.go's appendContext), which now grows unbounded across a
	// plan's checklist rather than being capped by the outer loop itself.
	// It becomes part of this call's untouchable leading messages below, so
	// if it's already large relative to this agent's own budget, compact it
	// down now — otherwise it could blow past this agent's context window
	// on the very first completion call, before the tries-based compaction
	// below ever gets a chance to run. Capping it at half this agent's
	// threshold leaves the other half for this call's own tool calls and
	// responses.
	if budget := agent.compactThreshold() / 2; len(promptContext) > budget {
		promptContext = chat(ctx, provider, contextCompactorAgentName, promptContext, "", "", false)
	}

	messages := []anyllm.Message{
		{Role: anyllm.RoleSystem, Content: agent.system},
		{Role: anyllm.RoleUser, Content: promptContext},
		{Role: anyllm.RoleUser, Content: prompt},
	}

	if checkpoint := getCheckpoint(planID); checkpoint != "" {
		messages = append(messages, anyllm.Message{
			Role:    anyllm.RoleUser,
			Content: fmt.Sprintf("Checkpoint from a previous turn on this task: %s", checkpoint),
		})
	}

	// leading is the number of messages that make up this call's fixed
	// context (system prompt, promptContext, prompt, and the checkpoint
	// reminder if one was present), which compaction must always preserve
	// untouched.
	leading := len(messages)

	// Run the ReAct loop appending each LLM response and the results of each
	// tool call to the message list.
	tries := 0
	for {
		resp, err := provider.Completion(ctx, anyllm.CompletionParams{
			Model:           agent.model,
			Messages:        messages,
			Tools:           agent.tools,
			ToolChoice:      "auto",
			Temperature:     &agent.temp,
			TopP:            &agent.topP,
			ReasoningEffort: agent.reasoning,
			ResponseFormat:  agent.responseFormat,
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
				var result string

				// task_checkpoint is scoped to this plan and records the
				// agent's own account of what to do next, so it's handled
				// here rather than through the stateless executeTool
				// dispatch. Every other tool call still gets a mechanical
				// checkpoint breadcrumb recorded after it runs, so there is
				// always something to recover from even if the agent never
				// checkpoints on its own.
				if tc.Function.Name == "task_checkpoint" {
					result = taskCheckpoint(planID, tc.Function.Arguments)
				} else {
					result = executeTool(agent, tc.Function.Name, tc.Function.Arguments)
					noteCheckpointAction(planID, tc.Function.Name, tc.Function.Arguments)
				}

				log.Debug().
					Str("function", tc.Function.Name).
					Str("arguments", tc.Function.Arguments).
					Msg(result)

				if verbose {
					snippet := result
					if len(snippet) > maxSnippet {
						snippet = snippet[:maxSnippet]
					}
					fmt.Printf("[%s] %s\n%s\n", agentName, tc.Function.Name, snippet)
				}

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
		if messagesSize(messages) > agent.compactThreshold() {
			messages = compactHistory(ctx, provider, messages, leading, planID)
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

// compactHistory replaces everything after the leading messages (the system
// prompt, the initial context and prompt, and the checkpoint reminder if one
// was present) with a single summary produced by the compactor agent, then
// re-appends the current checkpoint so the model isn't left to recover its
// task purely from that summary. The leading messages are always preserved
// untouched so the agent never loses its own instructions or task, only the
// accumulated tool calls and intermediate responses that led up to now.
func compactHistory(ctx context.Context, provider anyllm.Provider, messages []anyllm.Message, leading int, planID string) []anyllm.Message {
	// Nothing to compact yet: only the leading messages are present.
	if len(messages) <= leading {
		return messages
	}

	var history strings.Builder
	for _, m := range messages[leading:] {
		fmt.Fprintf(&history, "%s: %s\n", m.Role, contentStr(m.Content))
	}

	summary := chat(ctx, provider, contextCompactorAgentName, history.String(), "", "", false)

	log.Debug().
		Int("before", messagesSize(messages)).
		Str("summary", summary).
		Msg("compacted chat history")

	compacted := append([]anyllm.Message{}, messages[:leading]...)
	compacted = append(compacted, anyllm.Message{
		Role:    anyllm.RoleUser,
		Content: fmt.Sprintf("Summary of work completed so far:\n%s", summary),
	})

	if checkpoint := getCheckpoint(planID); checkpoint != "" {
		compacted = append(compacted, anyllm.Message{
			Role:    anyllm.RoleUser,
			Content: fmt.Sprintf("Checkpoint from a previous turn on this task: %s", checkpoint),
		})
	}

	return compacted
}
