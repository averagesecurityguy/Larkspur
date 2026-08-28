package larkspur

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
	ollamaapi "github.com/ollama/ollama/api"
	"github.com/rs/zerolog/log"
)

//go:embed prompts/* schemas/* tools/*
var assets embed.FS

type agent struct {
	model          string
	system         string
	tools          []anyllm.Tool
	temp           float64
	topP           float64
	reasoning      anyllm.ReasoningEffort
	maxTries       int
	responseFormat *anyllm.ResponseFormat

	// contextTokens is the context window available to this agent's model.
	// It's declared per agent, rather than assumed to be a single
	// provider-wide constant, because agents can be pointed at different
	// models with different windows. See compactThreshold (chat.go), which
	// is sized off this agent's own contextTokens.
	contextTokens int
}

// defaultModelContextTokens is the starting value for every agent's
// contextTokens, used until RefreshAgentContextWindows queries the real
// figure for each agent's model. It's only a fallback: if that query fails
// (ollama unreachable at agent-config time, model not yet pulled, an older
// ollama without model_info), an agent keeps this value rather than being
// left at zero.
const defaultModelContextTokens = 32000

var (
	// Load our JSON schemas, which are used to constrain the model responses
	// to match what we need. The raw string form is embedded in a prompt for
	// the model to read; the parsed map form is handed to the provider so it
	// can enforce the shape directly (see routerAgent's responseFormat).
	agentPlanSchema   = loadContent("schemas/agent_plan.json")
	planSummarySchema = loadContent("schemas/plan_summary.json")
	routeSchema       = loadContent("schemas/route.json")

	agentPlanSchemaMap   = loadSchema("schemas/agent_plan.json")
	planSummarySchemaMap = loadSchema("schemas/plan_summary.json")
	routeSchemaMap       = loadSchema("schemas/route.json")

	// Various agents
	developerAgentName = "developer"
	developerAgent     = &agent{
		model: "generalist",
		system: `
		You are a senior software engineer with expertise in multiple languages.
		You always write idiomatic, readable code and add appropriate comments
		using each languages preferred documentation style.

		There is no Python, Node, or other general-purpose scripting language
		available to you, and system_command cannot run one — do not try
		"python", "python3", "node", or writing a script to a file and
		executing it. For counting, sums, sorting, JSON or text
		transformation, or any other computation you would normally reach
		for a quick script to do, use the run_starlark tool instead: it runs
		real, deterministic code with no shell or interpreter involved.
		`,
		tools:         loadExecutionTools(),
		temp:          0.3,
		topP:          0.9,
		reasoning:     anyllm.ReasoningEffortMedium,
		maxTries:      8,
		contextTokens: defaultModelContextTokens,
	}

	generalistAgentName = "generalist"
	generalistAgent     = &agent{
		model: "generalist",
		system: `
		You are a helpful assistant.

		There is no Python or shell scripting language available to you.
		For counting, sums, sorting, JSON or text transformation, or any
		other computation you would normally reach for a quick script to
		do, use the run_starlark tool instead of system_command.
		`,
		tools:         loadExecutionTools(),
		temp:          0.7,
		topP:          0.95,
		reasoning:     anyllm.ReasoningEffortNone,
		maxTries:      5,
		contextTokens: defaultModelContextTokens,
	}

	routerAgentName = "router"
	routerSystem    = `
		You analyze a user's request to determine if you can answer it
		directly or if a plan needs to be created first. Simple requests
		like "what is the capital of France" or "why is the sky blue" can
		be answered by you. More complex requests like "Write a python
		script" or "analyze this source code" require a plan and you must
		not answer directly. Respond with a JSON object that matches the
		following schema:
		`
	routerAgent = &agent{
		model:         "generalist",
		system:        fmt.Sprintf("%s\n%s\n", routerSystem, routeSchema),
		temp:          0.3,
		topP:          0.9,
		reasoning:     anyllm.ReasoningEffortNone,
		maxTries:      2,
		contextTokens: defaultModelContextTokens,
		responseFormat: &anyllm.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &anyllm.JSONSchema{
				Name:   "route_decision",
				Schema: routeSchemaMap,
			},
		},
	}

	contextCompactorAgentName = "compactor"
	contextCompactorAgent     = &agent{
		model: "generalist",
		system: `
		You faithfully summarize the given content ensuring only the most valuable
		information is kept. Your summaries will be read by other LLM agents.
		`,
		temp:          0.3,
		topP:          0.9,
		reasoning:     anyllm.ReasoningEffortLow,
		maxTries:      2,
		contextTokens: defaultModelContextTokens,
	}

	planSummarizerAgentName = "summarizer"
	planSummarizerSystem    = loadContent("prompts/plan_summarizer.md")
	planSummarizerAgent     = &agent{
		model:         "generalist",
		system:        fmt.Sprintf("%s\n%s\n", planSummarizerSystem, planSummarySchema),
		temp:          0.4,
		topP:          0.9,
		reasoning:     anyllm.ReasoningEffortLow,
		maxTries:      2,
		contextTokens: defaultModelContextTokens,
		responseFormat: &anyllm.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &anyllm.JSONSchema{
				Name:   "plan_summary",
				Schema: planSummarySchemaMap,
			},
		},
	}

	planCreatorAgentName = "creator"
	planCreatorSystem    = loadContent("prompts/plan_creator.md")
	planCreatorAgent     = &agent{
		model:         "generalist",
		system:        fmt.Sprintf("%s\n%s\n", planCreatorSystem, agentPlanSchema),
		tools:         loadAllTools(),
		temp:          0.2,
		topP:          0.9,
		reasoning:     anyllm.ReasoningEffortMedium,
		maxTries:      4,
		contextTokens: defaultModelContextTokens,
		// format only constrains the model's final content, not its tool
		// calls (verified against a live ollama instance), so this is safe
		// to combine with tools: the creator can still call memory_search
		// mid-loop and only has to match the schema once it's done and
		// ready to hand back the plan.
		responseFormat: &anyllm.ResponseFormat{
			Type: "json_schema",
			JSONSchema: &anyllm.JSONSchema{
				Name:   "agent_plan",
				Schema: agentPlanSchemaMap,
			},
		},
	}

	// planVerifierAgent checks one checklist item at a time against work
	// already completed. The claims it checks are deliberately narrow and
	// mechanical, so it runs with low reasoning effort and a low
	// temperature — there's little to reason about, and speed matters more
	// than creativity here. maxTries is sized for the most demanding case
	// (a build/test/lint item needing several tool calls), not the typical
	// one- or two-call case.
	planVerifierAgentName = "verifier"
	planVerifierSystem    = loadContent("prompts/plan_verifier.md")
	planVerifierAgent     = &agent{
		model:         "generalist",
		system:        planVerifierSystem,
		tools:         loadAllTools(),
		temp:          0.2,
		topP:          0.9,
		reasoning:     anyllm.ReasoningEffortNone,
		maxTries:      5,
		contextTokens: defaultModelContextTokens,
	}
)

// allAgents lists every configured agent so RefreshAgentContextWindows can
// query and update each one's contextTokens.
var allAgents = []*agent{
	developerAgent,
	generalistAgent,
	routerAgent,
	contextCompactorAgent,
	planSummarizerAgent,
	planCreatorAgent,
	planVerifierAgent,
}

// RefreshAgentContextWindows queries ollamaHost's native API — not the
// OpenAI-compatible endpoint chat completions go through, which doesn't
// expose this — for each distinct model configured across allAgents, and
// sets that agent's contextTokens to what the model actually reports
// instead of defaultModelContextTokens. Each agent's compactThreshold
// (chat.go) reads its own contextTokens live on every call, so nothing else
// needs recomputing once this returns.
//
// A model that can't be queried (not pulled yet, no num_ctx set, a served
// context window below minModelContextTokens, an unexpected response shape)
// keeps its current contextTokens rather than failing the whole call: a
// possibly-stale budget is better than blocking startup over one agent's
// metadata.
func RefreshAgentContextWindows(ollamaHost string) error {
	parsed, err := url.Parse(ollamaHost)
	if err != nil {
		return fmt.Errorf("invalid ollama host %q: %w", ollamaHost, err)
	}

	client := ollamaapi.NewClient(parsed, http.DefaultClient)

	seen := make(map[string]int)

	for _, a := range allAgents {
		if tokens, ok := seen[a.model]; ok {
			a.contextTokens = tokens
			continue
		}

		tokens, err := queryContextTokens(client, a.model)
		if err != nil {
			log.Warn().Err(err).Str("model", a.model).Msg("could not query model context length, keeping current value")
			continue
		}

		a.contextTokens = tokens
		seen[a.model] = tokens
	}

	return nil
}

// minModelContextTokens is the smallest context window
// RefreshAgentContextWindows will accept from a provider. A model reporting
// less than this is treated as misconfigured rather than trusted, since
// Larkspur's prompts (e.g. prompts/plan_creator.md) are sized assuming at
// least this much room.
const minModelContextTokens = 32768

// queryContextTokens asks ollama, via its native /api/show endpoint, for the
// context window it is actually serving model at — the num_ctx parameter
// applied when the model was created (see modelfiles/generalist.Modelfile) —
// rather than the model's trained maximum, since Ollama serves at num_ctx
// (or a 4096 default when none is set) regardless of how large that trained
// maximum is. Returns an error if num_ctx isn't set or falls below
// minModelContextTokens.
func queryContextTokens(client *ollamaapi.Client, model string) (int, error) {
	resp, err := client.Show(context.Background(), &ollamaapi.ShowRequest{Model: model})
	if err != nil {
		return 0, fmt.Errorf("show %s: %w", model, err)
	}

	numCtx, ok := parseNumCtx(resp.Parameters)
	if !ok {
		return 0, fmt.Errorf("show %s: no num_ctx set", model)
	}

	if numCtx < minModelContextTokens {
		return 0, fmt.Errorf("show %s: num_ctx %d is below the minimum %d Larkspur expects", model, numCtx, minModelContextTokens)
	}

	return numCtx, nil
}

// parseNumCtx looks for a num_ctx line in the raw parameter listing
// ShowResponse.Parameters returns (one "key value" pair per line, key
// left-padded), such as the one num_ctx PARAMETER in a Modelfile produces.
func parseNumCtx(parameters string) (int, bool) {
	for _, line := range strings.Split(parameters, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "num_ctx" {
			continue
		}

		numCtx, err := strconv.Atoi(fields[1])
		if err != nil {
			return 0, false
		}

		return numCtx, true
	}

	return 0, false
}

// loadContent loads the content of the given path or exits on failure.
func loadContent(path string) string {
	data, err := assets.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not load prompt")
	}

	return string(data)
}

// loadSchema loads and parses the JSON schema at the given path, for use as
// a provider-enforced response format, or exits on failure.
func loadSchema(path string) map[string]any {
	var schema map[string]any

	data, err := assets.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not load schema")
	}

	if err := json.Unmarshal(data, &schema); err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("could not parse schema")
	}

	return schema
}

// getAgent gets the correct agent by name
func getAgent(name string) *agent {
	log.Debug().Msgf("getting agent for %s", name)

	switch name {
	case routerAgentName:
		return routerAgent
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
