package larkspur

import (
	anyllm "github.com/mozilla-ai/any-llm-go"
)

var (
	routerModel = "gemma4:e2b"
	routerPrompt = `
	You analyze a user's request to understand their goal then you route the
	request to the best available agent to complete the task. You do not
	attempt to answer the user's request directly, you return a RouteResponse
	that contains the agent and the prompt for the agent.

	# Available Agents
	- **developer** - If the user's request requires any software development
	tasks such as reading, writing, or analyzing programs, scripts, functions,
	or software repository contents, route the request to the 'developer'
	agent.
	- **generalist** - If the user's request is not better served by one of the
	other agents, route it to the 'generalist' agent.

	# Creating a Prompt
	Sometimes the user's request isn't detailed enough and you may need to
	modify it to further explain to the agent what the user wants to achieve.
	If you need to modify the user's request you should add sufficient detail
	without being overly verbose.
	`
	routerSchemaStrict = true
	routerResponse = &anyllm.ResponseFormat{
        Type: "json_schema",
        JSONSchema: &anyllm.JSONSchema{
            Name:   "RouteResponse",
            Strict: &routerSchemaStrict,
            Schema: map[string]any{
                "type": "object",
                "properties": map[string]any{
                    "agent":  map[string]any{"type": "string"},
                    "prompt": map[string]any{"type": "string"},
                },
                "required": []string{"agent", "prompt"},
            },
        },
    }
)





