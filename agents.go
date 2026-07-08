package larkspur

import (
	anyllm "github.com/mozilla-ai/any-llm-go"
)

type agent struct {
	model  string
	system string
	tools  []anyllm.Tool
}

var developerAgent = &agent{
	model: "qwen3.5:0.8b",
	system: `
	You are a senior software engineer with expertise in multiple languages.
	You always write idiomatic, readable code and add appropriate comments
	using each languages preferred documentation style.
	`,
	tools: loadAllTools(),
}

var generalistAgent = &agent{
	model:  "llama3.2",
	system: `You are a helpful assistant.`,
	tools:  loadAllTools(),
}

func getAgent(name string) *agent {
	switch name {
	case "developer":
		return developerAgent
	default:
		return generalistAgent
	}
}
