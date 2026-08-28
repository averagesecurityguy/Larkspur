# Larkspur

## Ollama setup

Ollama serves a model with a runtime context window of 4096 tokens by
default, unless told otherwise. Larkspur reads back whatever window
Ollama actually reports (its num_ctx, not the model's trained maximum)
and sizes every agent's compaction thresholds off of it — see
`RefreshAgentContextWindows` in `agents.go`. Larkspur requires at least
32768; a model reporting less is treated as misconfigured rather than
trusted. Left below that, larger prompts (e.g.
`prompts/plan_creator.md`) can get silently truncated mid-generation
instead of ever tripping Larkspur's own compaction.

Before running Larkspur against Ollama, build the `generalist` model tag
every agent is configured to use (see `agents.go`) from the Modelfile
checked into `modelfiles/`, so the served context window is raised
above that default:

```
ollama create generalist -f modelfiles/generalist.Modelfile
```

Confirm it took effect with `ollama show generalist` (or `ollama ps`
after sending it a request), which should report a context window of
32768. Re-run the `ollama create` command any time gemma4:e2b is
re-pulled, since re-pulling that base model does not automatically
propagate to the `generalist` tag built on top of it.