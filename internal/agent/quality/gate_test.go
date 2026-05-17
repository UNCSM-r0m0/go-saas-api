package quality

import (
	"context"
	"strings"
	"testing"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
	"go.uber.org/zap"
)

type mockLLM struct {
	response string
}

func (m *mockLLM) Stream(_ context.Context, _ llm.Request) (<-chan llm.Chunk, error) {
	ch := make(chan llm.Chunk)
	close(ch)
	return ch, nil
}
func (m *mockLLM) Complete(_ context.Context, _ llm.Request) (string, error) {
	return m.response, nil
}
func (m *mockLLM) HealthCheck(_ context.Context) error { return nil }

func TestShouldEvaluate_CoderWithCode(t *testing.T) {
	agent := &model.Agent{Role: model.RoleCoder}
	if !ShouldEvaluate(agent, "```go\nfunc main() {}\n```") {
		t.Error("expected ShouldEvaluate=true for coder with code")
	}
}

func TestShouldEvaluate_Assistant(t *testing.T) {
	agent := &model.Agent{Role: model.RoleAssistant}
	if ShouldEvaluate(agent, "```go\nfunc main() {}\n```") {
		t.Error("expected ShouldEvaluate=false for assistant")
	}
}

func TestShouldEvaluate_NoCode(t *testing.T) {
	agent := &model.Agent{Role: model.RoleCoder}
	if ShouldEvaluate(agent, "Hello, how are you?") {
		t.Error("expected ShouldEvaluate=false for text without code")
	}
}

func TestGate_Evaluate_Pass(t *testing.T) {
	mock := &mockLLM{response: `{"regenerate": false, "reason": "", "scores": {"completeness": 9, "correctness": 9, "safety": 8}}`}
	gate := NewGate(mock, logger.Logger{Logger: zap.NewNop()})
	eval, err := gate.Evaluate(context.Background(), "write a hello world", "```go\nfunc main() {}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Regenerate {
		t.Error("expected regenerate=false")
	}
	if eval.Scores.Completeness != 9 {
		t.Fatalf("expected completeness=9, got %d", eval.Scores.Completeness)
	}
}

func TestGate_Evaluate_Fail(t *testing.T) {
	mock := &mockLLM{response: `{"regenerate": true, "reason": "missing error handling", "scores": {"completeness": 5, "correctness": 9, "safety": 6}}`}
	gate := NewGate(mock, logger.Logger{Logger: zap.NewNop()})
	eval, err := gate.Evaluate(context.Background(), "write a hello world", "```go\nfunc main() {}\n```")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !eval.Regenerate {
		t.Error("expected regenerate=true")
	}
	if !strings.Contains(eval.Reason, "missing error handling") {
		t.Fatalf("expected reason to mention missing error handling, got %q", eval.Reason)
	}
}

func TestGate_Evaluate_NilClient(t *testing.T) {
	gate := NewGate(nil, logger.Logger{Logger: zap.NewNop()})
	eval, err := gate.Evaluate(context.Background(), "write a hello world", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eval.Regenerate {
		t.Error("expected regenerate=false when client is nil")
	}
}
