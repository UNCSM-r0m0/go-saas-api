package runtime

import (
	"testing"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
)

func TestClassify(t *testing.T) {
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
			got := Classify(tt.input)
			if got != tt.expected {
				t.Fatalf("Classify(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
