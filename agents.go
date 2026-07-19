package larkspur

import (
	"fmt"
	"os"
	"path/filepath"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

type agent struct {
	model     string
	system    string
	tools     []anyllm.Tool
	temp      float64
	topP      float64
	reasoning anyllm.ReasoningEffort
}

var (
	// Plan Schema
	agentPlanSchema = loadContent("schemas/agent_plan.json")

	// Various agents
	developerAgentName = "developer"
	developerAgent     = &agent{
		model: "qwen3.5:0.8b",
		system: `
		You are a senior software engineer with expertise in multiple languages.
		You always write idiomatic, readable code and add appropriate comments
		using each languages preferred documentation style.
		`,
		tools:     loadAllTools(),
		temp:      1.0,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortHigh,
	}

	generalistAgentName = "generalist"
	generalistAgent     = &agent{
		model:     "gemma4:e2b",
		system:    `You are a helpful assistant.`,
		tools:     loadAllTools(),
		temp:      1.0,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortHigh,
	}

	contextCompactorAgentName = "compactor"
	contextCompactorAgent     = &agent{
		model: "gemma4:e2b",
		system: `
		You faithfully summarize the given content ensuring only the most valuable
		information is kept. Your summaries will be read by other LLM agents.
		`,
		temp:      1.0,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortLow,
	}

	planSummarizerAgentName = "summarizer"
	planSummarizerAgent     = &agent{
		model:     "gemma4:e2b",
		system:    loadContent("prompts/plan_summarizer.md"),
		temp:      1.0,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortLow,
	}

	planCreatorAgentName = "creator"
	planCreatorSystem    = loadContent("prompts/plan_creator.md")
	planCreatorAgent     = &agent{
		model:     "gemma4:e2b",
		system:    fmt.Sprintf("%s\n%s\n", planCreatorSystem, agentPlanSchema),
		tools:     loadAllTools(),
		temp:      1.0,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortHigh,
	}

	planVerifierAgentName = "verifier"
	planVerifierSystem    = loadContent("prompts/plan_verifier.md")
	planVerifierAgent     = &agent{
		model:     "gemma4:e2b",
		system:    fmt.Sprintf("%s\n%s\n", planVerifierSystem, agentPlanSchema),
		tools:     loadAllTools(),
		temp:      1.0,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortHigh,
	}
)

// loadContent loads the content of the given path or exits on failure.
func loadContent(path string) string {
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not load prompt")
	}

	return string(data)
}

// getAgent gets the correct agent by name
func getAgent(name string) *agent {
	log.Debug().Msgf("getting agent for %s", name)

	switch name {
	case planCreatorAgentName:
		return planCreatorAgent
	case planVerifierAgentName:
		return planVerifierAgent
	case planSummarizerAgentName:
		return planSummarizerAgent
	case contextCompactorAgentName:
		return contextCompactorAgent
	case developerAgentName:
		return developerAgent
	default:
		return generalistAgent
	}
}
