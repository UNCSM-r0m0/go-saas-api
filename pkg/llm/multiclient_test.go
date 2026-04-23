package llm

import (
	"context"
	"errors"
	"testing"
	"time"
)

type mockClient struct {
	chunks []Chunk
	err    error
}

func (m *mockClient) Stream(_ context.Context, _ Request) (<-chan Chunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan Chunk)
	go func() {
		defer close(ch)
		for _, c := range m.chunks {
			ch <- c
		}
	}()
	return ch, nil
}

func (m *mockClient) HealthCheck(_ context.Context) error { return nil }

func TestMultiClient_RegisterAndList(t *testing.T) {
	mc := NewMultiClient()

	mc.Register(ProviderConfig{
		Name:    "p1",
		Models:  []string{"model-a"},
		Client:  &mockClient{},
		Enabled: true,
		Weight:  10,
	})

	providers := mc.ListProviders()
	if len(providers) != 1 {
		t.Fatalf("expected 1 provider, got %d", len(providers))
	}
	if providers[0].Name != "p1" {
		t.Fatalf("expected p1, got %s", providers[0].Name)
	}
}

func TestMultiClient_Stream_ModelMapped(t *testing.T) {
	mc := NewMultiClient()

	mc.Register(ProviderConfig{
		Name:    "p1",
		Models:  []string{"model-a"},
		Client:  &mockClient{chunks: []Chunk{{Content: "from p1"}}},
		Enabled: true,
		Weight:  10,
	})
	mc.Register(ProviderConfig{
		Name:    "p2",
		Models:  []string{"model-b"},
		Client:  &mockClient{chunks: []Chunk{{Content: "from p2"}}},
		Enabled: true,
		Weight:  20,
	})

	ch, err := mc.Stream(context.Background(), Request{Model: "model-b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got string
	for c := range ch {
		got += c.Content
	}
	if got != "from p2" {
		t.Fatalf("expected 'from p2', got %q", got)
	}
}

func TestMultiClient_Stream_Fallback(t *testing.T) {
	mc := NewMultiClient()

	mc.Register(ProviderConfig{
		Name:    "p1",
		Models:  []string{"model-x"},
		Client:  &mockClient{err: errors.New("p1 down")},
		Enabled: true,
		Weight:  100,
	})
	mc.Register(ProviderConfig{
		Name:    "p2",
		Models:  []string{"model-x"},
		Client:  &mockClient{chunks: []Chunk{{Content: "from p2"}}},
		Enabled: true,
		Weight:  50,
	})

	ch, err := mc.Stream(context.Background(), Request{Model: "model-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got string
	for c := range ch {
		got += c.Content
	}
	if got != "from p2" {
		t.Fatalf("expected 'from p2', got %q", got)
	}
}

func TestMultiClient_Stream_AllFail(t *testing.T) {
	mc := NewMultiClient()

	mc.Register(ProviderConfig{
		Name:    "p1",
		Models:  []string{"model-x"},
		Client:  &mockClient{err: errors.New("p1 down")},
		Enabled: true,
		Weight:  10,
	})

	_, err := mc.Stream(context.Background(), Request{Model: "model-x"})
	if err == nil {
		t.Fatal("expected error when all providers fail")
	}
}

func TestMultiClient_Stream_NoProviders(t *testing.T) {
	mc := NewMultiClient()
	_, err := mc.Stream(context.Background(), Request{Model: "anything"})
	if err == nil {
		t.Fatal("expected error with no providers")
	}
}

func TestMultiClient_HealthCheck(t *testing.T) {
	mc := NewMultiClient()

	mc.Register(ProviderConfig{
		Name:    "p1",
		Models:  []string{"m1"},
		Client:  &mockClient{},
		Enabled: true,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := mc.HealthCheck(ctx); err != nil {
		t.Fatalf("unexpected health check error: %v", err)
	}
}

func TestMultiClient_DisabledProviderSkipped(t *testing.T) {
	mc := NewMultiClient()

	mc.Register(ProviderConfig{
		Name:    "p1",
		Models:  []string{"m1"},
		Client:  &mockClient{err: errors.New("p1 down")},
		Enabled: false,
		Weight:  100,
	})
	mc.Register(ProviderConfig{
		Name:    "p2",
		Models:  []string{"m1"},
		Client:  &mockClient{chunks: []Chunk{{Content: "ok"}}},
		Enabled: true,
		Weight:  10,
	})

	ch, err := mc.Stream(context.Background(), Request{Model: "m1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got string
	for c := range ch {
		got += c.Content
	}
	if got != "ok" {
		t.Fatalf("expected 'ok', got %q", got)
	}
}
