package llm

import "testing"

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"", 0},
		{"hello", 1},
		{"hello world", 2},
		{"this is a longer sentence with more words", 10},
		{string(make([]byte, 400)), 100},
	}

	for _, tt := range tests {
		got := EstimateTokens(tt.input)
		if got != tt.expected {
			t.Errorf("EstimateTokens(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

func TestEstimateRequestTokens(t *testing.T) {
	msgs := []Message{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello, how are you?"},
	}
	got := EstimateRequestTokens(msgs)
	// system: 28 chars / 4 = 7, user: 19 chars / 4 = 4
	want := 11
	if got != want {
		t.Errorf("EstimateRequestTokens() = %d, want %d", got, want)
	}
}
