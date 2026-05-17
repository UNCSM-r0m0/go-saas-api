package flows

import (
	"strings"
	"testing"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
)

func TestRegistry_RegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	f := NewOnboardingFlow()
	reg.Register(f)

	got, ok := reg.Get("onboarding")
	if !ok {
		t.Fatal("expected to find onboarding flow")
	}
	if got.Name() != "onboarding" {
		t.Fatalf("expected onboarding, got %s", got.Name())
	}
}

func TestRegistry_DetectAll_OnboardingFirstMessage(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewOnboardingFlow())

	f, score := reg.DetectAll("hello", 0)
	if f == nil {
		t.Fatal("expected onboarding flow to trigger on first message")
	}
	if f.Name() != "onboarding" {
		t.Fatalf("expected onboarding, got %s", f.Name())
	}
	if score < 0.9 {
		t.Fatalf("expected high confidence, got %f", score)
	}
}

func TestRegistry_DetectAll_Proposal(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewProposalFlow())

	f, score := reg.DetectAll("I need a proposal for my project", 2)
	if f == nil {
		t.Fatal("expected proposal flow to trigger")
	}
	if f.Name() != "proposal" {
		t.Fatalf("expected proposal, got %s", f.Name())
	}
	if score < 0.8 {
		t.Fatalf("expected high confidence, got %f", score)
	}
}

func TestRegistry_DetectAll_NoMatch(t *testing.T) {
	reg := NewRegistry()
	reg.Register(NewOnboardingFlow())
	reg.Register(NewProposalFlow())

	f, _ := reg.DetectAll("just a regular question", 5)
	if f != nil {
		t.Fatalf("expected no flow, got %s", f.Name())
	}
}

func TestOnboardingFlow_BuildMessages(t *testing.T) {
	f := NewOnboardingFlow()
	agent := &model.Agent{Role: model.RoleAssistant, SystemPrompt: "You are helpful."}
	msgs := f.BuildMessages(agent, nil, "hello", "", nil)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (system + user), got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatalf("expected first message system, got %s", msgs[0].Role)
	}
	if !contains(msgs[0].Content, "onboarding") {
		t.Error("expected system prompt to mention onboarding")
	}
	if msgs[1].Content != "hello" {
		t.Fatalf("expected user message hello, got %s", msgs[1].Content)
	}
}

func TestOnboardingFlow_UpdateState(t *testing.T) {
	f := NewOnboardingFlow()
	meta := f.UpdateState(nil, "What is your name?")
	if meta["flow"] != "onboarding" {
		t.Fatalf("expected flow=onboarding, got %v", meta["flow"])
	}
	if meta["flow_step"] != 1 {
		t.Fatalf("expected step=1, got %v", meta["flow_step"])
	}
}

func TestProposalFlow_Detect(t *testing.T) {
	f := NewProposalFlow()
	matched, score := f.Detect("Necesito un presupuesto", 0)
	if !matched {
		t.Error("expected proposal to match presupuesto")
	}
	if score < 0.8 {
		t.Fatalf("expected high score, got %f", score)
	}
}

func TestContractFlow_Detect(t *testing.T) {
	f := NewContractFlow()
	matched, score := f.Detect("draft a contract", 0)
	if !matched {
		t.Error("expected contract to match")
	}
	if score < 0.8 {
		t.Fatalf("expected high score, got %f", score)
	}
}

func TestCodeReviewFlow_Detect(t *testing.T) {
	f := NewCodeReviewFlow()
	matched, score := f.Detect("review my code", 0)
	if !matched {
		t.Error("expected code_review to match")
	}
	if score < 0.8 {
		t.Fatalf("expected high score, got %f", score)
	}
}

func TestFlowMessagesStructure(t *testing.T) {
	// Ensure all flows produce valid message structures
	flowList := []Flow{
		NewOnboardingFlow(),
		NewProposalFlow(),
		NewContractFlow(),
		NewCodeReviewFlow(),
	}

	agent := &model.Agent{Role: model.RoleAssistant}
	history := []model.Message{
		{Role: model.MessageRoleUser, Content: "previous"},
		{Role: model.MessageRoleAssistant, Content: "response"},
	}

	for _, f := range flowList {
		msgs := f.BuildMessages(agent, history, "test", "context", nil)
		if len(msgs) < 2 {
			t.Fatalf("flow %s: expected at least 2 messages, got %d", f.Name(), len(msgs))
		}
		if msgs[0].Role != "system" {
			t.Fatalf("flow %s: expected first message system, got %s", f.Name(), msgs[0].Role)
		}
		// Verify user message is last
		last := msgs[len(msgs)-1]
		if last.Role != "user" || last.Content != "test" {
			t.Fatalf("flow %s: expected last message user:test, got %s:%s", f.Name(), last.Role, last.Content)
		}
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
