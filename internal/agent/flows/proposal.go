package flows

import (
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ProposalFlow guides users through creating a project proposal or quote.
type ProposalFlow struct{}

// NewProposalFlow creates a new proposal flow.
func NewProposalFlow() *ProposalFlow {
	return &ProposalFlow{}
}

// Name returns the flow identifier.
func (f *ProposalFlow) Name() string { return "proposal" }

// Detect identifies proposal/quote/budget requests.
func (f *ProposalFlow) Detect(message string, historyLen int) (bool, float64) {
	lower := strings.ToLower(message)
	keywords := []string{"proposal", "quote", "budget", "presupuesto", "cotización", "propuesta"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true, 0.85
		}
	}
	return false, 0
}

// SystemPrompt returns the proposal flow system prompt.
func (f *ProposalFlow) SystemPrompt() string {
	return `You are a business consultant helping the user create a professional proposal or quote.

WORKFLOW:
1. Gather requirements: scope, timeline, deliverables, budget range.
2. Ask clarifying questions if anything is ambiguous.
3. Present a structured proposal with:
   - Executive Summary
   - Scope of Work
   - Timeline
   - Pricing
   - Terms & Conditions
4. Use file_write to save the final proposal document.

Be professional, concise, and actionable.`
}

// BuildMessages constructs messages for the proposal flow.
func (f *ProposalFlow) BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, metadata map[string]any) []llm.Message {
	var messages []llm.Message

	systemContent := f.SystemPrompt()
	if agent.SystemPrompt != "" {
		systemContent = agent.SystemPrompt + "\n\n" + systemContent
	}
	if userContext != "" {
		systemContent += "\n" + userContext
	}

	messages = append(messages, llm.Message{Role: "system", Content: systemContent})

	for _, h := range history {
		switch h.Role {
		case model.MessageRoleUser, model.MessageRoleSystem:
			messages = append(messages, llm.Message{Role: string(h.Role), Content: h.Content})
		case model.MessageRoleAssistant:
			messages = append(messages, llm.Message{Role: "assistant", Content: h.Content})
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: userMessage})
	return messages
}

// UpdateState tracks proposal flow progress.
func (f *ProposalFlow) UpdateState(metadata map[string]any, assistantContent string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["flow"] = f.Name()
	return metadata
}
