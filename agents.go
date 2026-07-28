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
	maxTries  int
}

var (
	// Plan Schema
	agentPlanSchema = loadContent("schemas/agent_plan.json")

	// Plan Summary Schema
	planSummarySchema = loadContent("schemas/plan_summary.json")

	// Various agents
	developerAgentName = "developer"
	developerAgent     = &agent{
		model: "gemma4:e2b",
		system: `
		You are a senior software engineer with expertise in multiple languages.
		You always write idiomatic, readable code and add appropriate comments
		using each languages preferred documentation style.
		`,
		tools:     loadExecutionTools(),
		temp:      0.3,
		topP:      0.9,
		reasoning: anyllm.ReasoningEffortMedium,
		maxTries:  8,
	}

	generalistAgentName = "generalist"
	generalistAgent     = &agent{
		model:     "gemma4:e2b",
		system:    `You are a helpful assistant.`,
		tools:     loadExecutionTools(),
		temp:      0.7,
		topP:      0.95,
		reasoning: anyllm.ReasoningEffortNone,
		maxTries:  5,
	}

	contextCompactorAgentName = "compactor"
	contextCompactorAgent     = &agent{
		model: "gemma4:e2b",
		system: `
		You faithfully summarize the given content ensuring only the most valuable
		information is kept. Your summaries will be read by other LLM agents.
		`,
		temp:      0.3,
		topP:      0.9,
		reasoning: anyllm.ReasoningEffortLow,
		maxTries:  2,
	}

	planSummarizerAgentName = "summarizer"
	planSummarizerSystem    = loadContent("prompts/plan_summarizer.md")
	planSummarizerAgent     = &agent{
		model:     "gemma4:e2b",
		system:    fmt.Sprintf("%s\n%s\n", planSummarizerSystem, planSummarySchema),
		temp:      0.4,
		topP:      0.9,
		reasoning: anyllm.ReasoningEffortLow,
		maxTries:  2,
	}

	planCreatorAgentName = "creator"
	planCreatorSystem    = loadContent("prompts/plan_creator.md")
	planCreatorAgent     = &agent{
		model:     "gemma4:e2b",
		system:    fmt.Sprintf("%s\n%s\n", planCreatorSystem, agentPlanSchema),
		tools:     loadAllTools(),
		temp:      0.2,
		topP:      0.9,
		reasoning: anyllm.ReasoningEffortHigh,
		maxTries:  4,
	}

	planVerifierAgentName = "verifier"
	planVerifierSystem    = loadContent("prompts/plan_verifier.md")
	planVerifierAgent     = &agent{
		model:     "gemma4:e2b",
		system:    fmt.Sprintf("%s\n%s\n", planVerifierSystem, agentPlanSchema),
		tools:     loadAllTools(),
		temp:      0.3,
		topP:      0.9,
		reasoning: anyllm.ReasoningEffortHigh,
		maxTries:  4,
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
