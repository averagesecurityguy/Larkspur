package larkspur

import (
	"context"
	"fmt"
	"testing"

	anyllm "github.com/mozilla-ai/any-llm-go"
)

// fakeProvider is a minimal anyllm.Provider that plays back a scripted
// sequence of responses, one per call to Completion, without making any
// network calls. It lets tests exercise code that drives the chat loop
// (routing decisions, plan generation and execution) without needing a
// live model. Every response is returned with FinishReasonStop, so it only
// covers agents/turns that don't need to make tool calls first.
type fakeProvider struct {
	responses []string
	calls     int
	err       error
}

func (f *fakeProvider) Name() string { return "fake" }

func (f *fakeProvider) Completion(_ context.Context, _ anyllm.CompletionParams) (*anyllm.ChatCompletion, error) {
	if f.err != nil {
		return nil, f.err
	}

	if f.calls >= len(f.responses) {
		return nil, fmt.Errorf("fakeProvider: unexpected call %d, only %d response(s) scripted", f.calls+1, len(f.responses))
	}

	content := f.responses[f.calls]
	f.calls++

	return &anyllm.ChatCompletion{
		Choices: []anyllm.Choice{{
			Message:      anyllm.Message{Role: anyllm.RoleAssistant, Content: content},
			FinishReason: anyllm.FinishReasonStop,
		}},
	}, nil
}

func (f *fakeProvider) CompletionStream(_ context.Context, _ anyllm.CompletionParams) (<-chan anyllm.ChatCompletionChunk, <-chan error) {
	ch := make(chan anyllm.ChatCompletionChunk)
	errCh := make(chan error)
	close(ch)
	close(errCh)

	return ch, errCh
}

type promptRouterTest struct {
	name         string
	content      string
	wantAnswer   string
	wantNeedPlan bool
}

func TestPromptRouter(t *testing.T) {
	tests := []promptRouterTest{
		{
			name:         "direct answer",
			content:      `{"answer": "Paris", "need_plan": false}`,
			wantAnswer:   "Paris",
			wantNeedPlan: false,
		},
		{
			name:         "needs a plan",
			content:      `{"answer": "", "need_plan": true}`,
			wantAnswer:   "",
			wantNeedPlan: true,
		},
		{
			name:         "strips json code fences",
			content:      "```json\n" + `{"answer": "blue light scatters more", "need_plan": false}` + "\n```",
			wantAnswer:   "blue light scatters more",
			wantNeedPlan: false,
		},
		{
			name:         "falls back to planning on unparsable output",
			content:      "not json",
			wantAnswer:   "",
			wantNeedPlan: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			provider := &fakeProvider{responses: []string{test.content}}

			answer, needPlan := promptRouter(context.Background(), provider, "some request", "")

			if answer != test.wantAnswer {
				t.Fatalf("Expected answer `%s`, received `%s`", test.wantAnswer, answer)
			}

			if needPlan != test.wantNeedPlan {
				t.Fatalf("Expected need_plan %v, received %v", test.wantNeedPlan, needPlan)
			}
		})
	}
}
