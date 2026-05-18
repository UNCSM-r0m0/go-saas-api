package llm

import (
	"context"
	"testing"
)

func TestKimiAnthropicClient_Complete(t *testing.T) {
	t.Skip("external integration test - requires valid live API key")
	client := NewKimiAnthropicClient("sk-kimi-gLUo4TLSgpz1UgkTSZZbxc3eWFbqHF3gawiXKwj5THA3UVarTrZe6sJCyIfOaFgG")
	ctx := context.Background()
	req := Request{
		Model: "kimi-for-coding",
		Messages: []Message{
			{Role: "user", Content: "Hola"},
		},
		MaxTokens: 4096,
	}
	resp, err := client.Complete(ctx, req)
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}
	t.Logf("Response: %s", resp)
	if resp == "" {
		t.Fatal("Expected non-empty response")
	}
}

func TestKimiAnthropicClient_Stream(t *testing.T) {
	t.Skip("external integration test - requires valid live API key")
	client := NewKimiAnthropicClient("sk-kimi-gLUo4TLSgpz1UgkTSZZbxc3eWFbqHF3gawiXKwj5THA3UVarTrZe6sJCyIfOaFgG")
	ctx := context.Background()
	req := Request{
		Model: "kimi-for-coding",
		Messages: []Message{
			{Role: "user", Content: "Hola"},
		},
		MaxTokens: 4096,
	}
	ch, err := client.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream failed: %v", err)
	}
	var full string
	for chunk := range ch {
		full += chunk.Content
		if chunk.Done {
			break
		}
	}
	t.Logf("Stream response: %s", full)
	if full == "" {
		t.Fatal("Expected non-empty stream response")
	}
}
