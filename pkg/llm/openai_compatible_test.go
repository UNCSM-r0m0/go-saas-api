package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestOpenAICompatibleClient_Stream(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		lines := []string{
			`data: {"choices":[{"delta":{"content":"Hello"}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"content":" world"}}]}` + "\n\n",
			"data: [DONE]\n\n",
		}
		for _, line := range lines {
			w.Write([]byte(line))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "test-key")
	ch, err := client.Stream(context.Background(), Request{
		Model:    "gpt-4o-mini",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result string
	for chunk := range ch {
		if chunk.Done {
			break
		}
		result += chunk.Content
	}
	if result != "Hello world" {
		t.Fatalf("expected 'Hello world', got %q", result)
	}
}

func TestOpenAICompatibleClient_Stream_WithoutAPIKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "expected no auth", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("data: [DONE]\n\n"))
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "")
	ch, err := client.Stream(context.Background(), Request{
		Model:    "model",
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for range ch {
	}
}

func TestOpenAICompatibleClient_Stream_StatusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "key")
	_, err := client.Stream(context.Background(), Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestOpenAICompatibleClient_HealthCheck(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAICompatibleClient_HealthCheck_Fail(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "key")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.HealthCheck(ctx); err == nil {
		t.Fatal("expected error for 500 status")
	}
}

func TestNewOpenAIClient(t *testing.T) {
	c := NewOpenAIClient("key")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	// Verify base URL is set to OpenAI default
	if c.baseURL != "https://api.openai.com/v1" {
		t.Fatalf("expected OpenAI base URL, got %s", c.baseURL)
	}
	if c.apiKey != "key" {
		t.Fatalf("expected key, got %s", c.apiKey)
	}
}

func TestNewDeepSeekClient(t *testing.T) {
	c := NewDeepSeekClient("key")
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.baseURL != "https://api.deepseek.com/v1" {
		t.Fatalf("expected DeepSeek base URL, got %s", c.baseURL)
	}
}

func TestOpenAICompatibleClient_Stream_ToolCall(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		// Verify tools are passed in request
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		tools, ok := body["tools"].([]any)
		if !ok || len(tools) == 0 {
			http.Error(w, "missing tools", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		lines := []string{
			`data: {"choices":[{"delta":{"content":"I will "}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call_123","type":"function","function":{"name":"echo","arguments":""}}]}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"msg\":\""}}]}}]}` + "\n\n",
			`data: {"choices":[{"delta":{"tool_calls":[{"index":0,"function":{"arguments":"ping\"}"}}]}}]}` + "\n\n",
			`data: {"choices":[{"delta":{},"finish_reason":"tool_calls"}]}` + "\n\n",
		}
		for _, line := range lines {
			w.Write([]byte(line))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "test-key")
	ch, err := client.Stream(context.Background(), Request{
		Model:    "gpt-4o-mini",
		Messages: []Message{{Role: "user", Content: "hi"}},
		Tools: []ToolDefinition{
			{Type: "function", Function: FunctionSchema{Name: "echo", Description: "echo", Parameters: map[string]any{"type": "object"}}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var result string
	var toolCall *ToolCall
	for chunk := range ch {
		result += chunk.Content
		if chunk.ToolCall != nil {
			toolCall = chunk.ToolCall
		}
		if chunk.Done {
			break
		}
	}

	if result != "I will " {
		t.Fatalf("expected 'I will ', got %q", result)
	}
	if toolCall == nil {
		t.Fatal("expected tool call in stream")
	}
	if toolCall.Name != "echo" {
		t.Fatalf("expected tool name echo, got %s", toolCall.Name)
	}
	if toolCall.Arguments["msg"] != "ping" {
		t.Fatalf("expected arg msg=ping, got %v", toolCall.Arguments)
	}
}

func TestOpenAICompatibleClient_Stream_ContextCancel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		for i := 0; i < 100; i++ {
			fmt.Fprintf(w, "data: {\"choices\":[{\"delta\":{\"content\":\"%d\"}}]}\n\n", i)
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer ts.Close()

	client := NewOpenAICompatibleClient(ts.URL, "")
	ctx, cancel := context.WithCancel(context.Background())

	ch, err := client.Stream(ctx, Request{Model: "m", Messages: []Message{{Role: "user", Content: "hi"}}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Cancel after receiving first chunk
	for chunk := range ch {
		if chunk.Content != "" {
			cancel()
			break
		}
	}
	// Should exit cleanly after cancel
}
