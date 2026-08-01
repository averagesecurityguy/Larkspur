package larkspur

import (
	"context"
	"encoding/json"
	"strings"

	anyllm "github.com/mozilla-ai/any-llm-go"
	"github.com/rs/zerolog/log"
)

type routeDecision struct {
	Answer   string `json:"answer"`
	NeedPlan bool   `json:"need_plan"`
}

// promptRouter determines if a given prompt needs to be routed to the planner
// or if it can be answered directly.
func promptRouter(ctx context.Context, provider anyllm.Provider, prompt, context string) (string, bool) {
	var decision routeDecision

	result := chat(ctx, provider, routerAgentName, prompt, context, "", false)

	// Trim any JSON markdown fencing
	result = strings.TrimPrefix(result, "```json")
	result = strings.TrimSuffix(result, "```")

	err := json.Unmarshal([]byte(result), &decision)
	if err != nil {
		// Fall back to routing through the planner rather than surfacing
		// this internal error as if it were an answer to the user.
		log.Error().Err(err).Str("raw", result).Msg("could not parse routing decision")
		return "", true
	}

	log.Debug().
		Str("answer", decision.Answer).
		Bool("need_plan", decision.NeedPlan).
		Msg("routing decision")

	return decision.Answer, decision.NeedPlan
}
