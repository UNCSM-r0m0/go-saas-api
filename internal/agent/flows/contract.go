package flows

import (
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// ContractFlow guides users through creating a contract or agreement.
type ContractFlow struct{}

// NewContractFlow creates a new contract flow.
func NewContractFlow() *ContractFlow {
	return &ContractFlow{}
}

// Name returns the flow identifier.
func (f *ContractFlow) Name() string { return "contract" }

// Detect identifies contract/agreement requests.
func (f *ContractFlow) Detect(message string, historyLen int) (bool, float64) {
	lower := strings.ToLower(message)
	keywords := []string{"contract", "agreement", "contrato", "acuerdo", "terms", "nda"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true, 0.85
		}
	}
	return false, 0
}

// SystemPrompt returns the contract flow system prompt.
func (f *ContractFlow) SystemPrompt() string {
	return `You are a legal assistant helping draft contracts and agreements.

WORKFLOW:
1. Identify the type of contract needed (service agreement, NDA, employment, etc.).
2. Gather key terms: parties, scope, payment, duration, termination, liability.
3. Draft a clear, professional contract.
4. Highlight areas that should be reviewed by a human lawyer.
5. Use file_write to save the final contract document.

IMPORTANT: Include a disclaimer that this is a draft and not legal advice.`
}

// BuildMessages constructs messages for the contract flow.
func (f *ContractFlow) BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, metadata map[string]any) []llm.Message {
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

// UpdateState tracks contract flow progress.
func (f *ContractFlow) UpdateState(metadata map[string]any, assistantContent string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["flow"] = f.Name()
	return metadata
}
