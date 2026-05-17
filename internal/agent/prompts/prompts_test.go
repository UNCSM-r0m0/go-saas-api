package prompts

import (
	"strings"
	"testing"
)

func TestCoder(t *testing.T) {
	p := Coder()
	if !strings.Contains(p, "senior software architect") {
		t.Error("expected coder prompt to mention 'senior software architect'")
	}
	if !strings.Contains(p, "file_write") {
		t.Error("expected coder prompt to mention file_write tool")
	}
	if !strings.Contains(p, "code_execute") {
		t.Error("expected coder prompt to mention code_execute tool")
	}
}

func TestConversational(t *testing.T) {
	p := Conversational()
	if !strings.Contains(p, "helpful") {
		t.Error("expected conversational prompt to mention 'helpful'")
	}
	if !strings.Contains(p, "available tools") {
		t.Error("expected conversational prompt to mention tools")
	}
}
