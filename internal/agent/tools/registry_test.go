package tools

import (
	"context"
	"testing"
)

type mockTool struct {
	name        string
	description string
	result      Result
	err         error
}

func (m *mockTool) Name() string        { return m.name }
func (m *mockTool) Description() string { return m.description }
func (m *mockTool) Execute(_ context.Context, _ map[string]any) (Result, error) {
	return m.result, m.err
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected non-nil registry")
	}
	if len(r.List()) != 0 {
		t.Fatalf("expected empty registry, got %d tools", len(r.List()))
	}
}

func TestRegisterAndGet(t *testing.T) {
	r := NewRegistry()
	mt := &mockTool{name: "test_tool", description: "a test tool"}

	if err := r.Register(mt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := r.Get("test_tool")
	if !ok {
		t.Fatal("expected tool to be found")
	}
	if got.Name() != "test_tool" {
		t.Fatalf("expected name test_tool, got %s", got.Name())
	}
}

func TestRegisterDuplicate(t *testing.T) {
	r := NewRegistry()
	mt := &mockTool{name: "dup"}
	if err := r.Register(mt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := r.Register(mt); err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

func TestRegisterEmptyName(t *testing.T) {
	r := NewRegistry()
	mt := &mockTool{name: ""}
	if err := r.Register(mt); err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestExecute(t *testing.T) {
	r := NewRegistry()
	mt := &mockTool{
		name:   "echo",
		result: Result{Content: "hello"},
	}
	r.Register(mt)

	res, err := r.Execute(context.Background(), "echo", map[string]any{"msg": "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != "hello" {
		t.Fatalf("expected hello, got %s", res.Content)
	}
}

func TestExecuteNotFound(t *testing.T) {
	r := NewRegistry()
	_, err := r.Execute(context.Background(), "missing", nil)
	if err == nil {
		t.Fatal("expected error for missing tool")
	}
}
