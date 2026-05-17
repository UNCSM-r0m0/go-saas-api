package runtime

import (
	"context"
	"errors"
	"testing"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ---- mock LLM for classifier ----

type mockClassifierLLM struct {
	response string
	err      error
}

func (m *mockClassifierLLM) Stream(_ context.Context, _ llm.Request) (<-chan llm.Chunk, error) {
	ch := make(chan llm.Chunk)
	close(ch)
	return ch, nil
}

func (m *mockClassifierLLM) Complete(_ context.Context, _ llm.Request) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.response, nil
}

func (m *mockClassifierLLM) HealthCheck(_ context.Context) error { return nil }

var _ llm.Client = (*mockClassifierLLM)(nil)

// ---- tests for ClassifyKeyword (legacy keyword matching) ----

func TestClassifyKeyword(t *testing.T) {
	tests := []struct {
		input    string
		expected model.AgentRole
	}{
		{"create a landing page with html and css", model.RoleCoder},
		{"write a python script to sort files", model.RoleCoder},
		{"generate code for a calculator", model.RoleCoder},
		{"research the latest AI models", model.RoleResearcher},
		{"busca información sobre golang", model.RoleResearcher},
		{"write a blog post about travel", model.RoleCopywriter},
		{"hello, how are you?", model.RoleAssistant},
		{"what is the weather?", model.RoleAssistant},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ClassifyKeyword(tt.input)
			if got != tt.expected {
				t.Fatalf("ClassifyKeyword(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

// ---- tests for LLMClassifier ----

func TestLLMClassifier_Classify_Coder(t *testing.T) {
	mock := &mockClassifierLLM{response: "coder"}
	classifier := NewLLMClassifier(mock)

	role, err := classifier.Classify(context.Background(), "write a python script")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != model.RoleCoder {
		t.Fatalf("expected coder, got %v", role)
	}
}

func TestLLMClassifier_Classify_Researcher(t *testing.T) {
	mock := &mockClassifierLLM{response: "researcher"}
	classifier := NewLLMClassifier(mock)

	role, err := classifier.Classify(context.Background(), "find information about golang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != model.RoleResearcher {
		t.Fatalf("expected researcher, got %v", role)
	}
}

func TestLLMClassifier_Classify_Copywriter(t *testing.T) {
	mock := &mockClassifierLLM{response: "copywriter"}
	classifier := NewLLMClassifier(mock)

	role, err := classifier.Classify(context.Background(), "write a blog post")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != model.RoleCopywriter {
		t.Fatalf("expected copywriter, got %v", role)
	}
}

func TestLLMClassifier_Classify_Assistant(t *testing.T) {
	mock := &mockClassifierLLM{response: "assistant"}
	classifier := NewLLMClassifier(mock)

	role, err := classifier.Classify(context.Background(), "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != model.RoleAssistant {
		t.Fatalf("expected assistant, got %v", role)
	}
}

func TestLLMClassifier_Classify_CaseInsensitive(t *testing.T) {
	mock := &mockClassifierLLM{response: "  CODER  "}
	classifier := NewLLMClassifier(mock)

	role, err := classifier.Classify(context.Background(), "write code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != model.RoleCoder {
		t.Fatalf("expected coder, got %v", role)
	}
}

func TestLLMClassifier_Classify_UnknownRole(t *testing.T) {
	mock := &mockClassifierLLM{response: "wizard"}
	classifier := NewLLMClassifier(mock)

	role, err := classifier.Classify(context.Background(), "cast a spell")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if role != model.RoleAssistant {
		t.Fatalf("expected assistant fallback for unknown role, got %v", role)
	}
}

func TestLLMClassifier_Classify_Error(t *testing.T) {
	mock := &mockClassifierLLM{err: errors.New("llm unavailable")}
	classifier := NewLLMClassifier(mock)

	_, err := classifier.Classify(context.Background(), "write code")
	if err == nil {
		t.Fatal("expected error when LLM fails")
	}
}

// ---- backward compatibility ----

func TestClassify_BackwardCompatibility(t *testing.T) {
	// Classify should still work as an alias for ClassifyKeyword
	got := Classify("write html")
	if got != model.RoleCoder {
		t.Fatalf("Classify(%q) = %v, want %v", "write html", got, model.RoleCoder)
	}
}
