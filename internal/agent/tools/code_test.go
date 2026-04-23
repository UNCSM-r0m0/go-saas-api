package tools

import (
	"context"
	"errors"
	"testing"
)

type mockSandbox struct {
	output string
	err    error
}

func (m *mockSandbox) Execute(_ context.Context, code, language string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return m.output, nil
}

func TestCodeExecuteTool_Execute(t *testing.T) {
	client := &mockSandbox{output: "42"}
	tool := NewCodeExecuteTool(client)

	res, err := tool.Execute(context.Background(), map[string]any{
		"code":     "print(42)",
		"language": "python",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "42" {
		t.Fatalf("expected 42, got %s", res.Content)
	}
}

func TestCodeExecuteTool_MissingCode(t *testing.T) {
	client := &mockSandbox{}
	tool := NewCodeExecuteTool(client)

	_, err := tool.Execute(context.Background(), map[string]any{
		"language": "python",
	})
	if err == nil {
		t.Fatal("expected error for missing code")
	}
}

func TestCodeExecuteTool_SandboxError(t *testing.T) {
	client := &mockSandbox{err: errors.New("sandbox timeout")}
	tool := NewCodeExecuteTool(client)

	res, err := tool.Execute(context.Background(), map[string]any{
		"code": "while True: pass",
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if res.Error != "sandbox timeout" {
		t.Fatalf("expected sandbox timeout, got %s", res.Error)
	}
}
