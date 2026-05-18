package flows

import (
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// CodeReviewFlow guides users through a structured code review.
type CodeReviewFlow struct{}

// NewCodeReviewFlow creates a new code review flow.
func NewCodeReviewFlow() *CodeReviewFlow {
	return &CodeReviewFlow{}
}

// Name returns the flow identifier.
func (f *CodeReviewFlow) Name() string { return "code_review" }

// Detect identifies code review/audit requests.
func (f *CodeReviewFlow) Detect(message string, historyLen int) (bool, float64) {
	lower := strings.ToLower(message)
	keywords := []string{"code review", "audit", "revisar código", "review my code", "audit my"}
	for _, kw := range keywords {
		if strings.Contains(lower, kw) {
			return true, 0.9
		}
	}
	return false, 0
}

// SystemPrompt returns the code review flow system prompt.
func (f *CodeReviewFlow) SystemPrompt() string {
	return `You are a senior code reviewer conducting a structured audit.

WORKFLOW:
1. Ask the user to share the code (file upload or paste).
2. Analyze the code for:
   - Bugs and logic errors
   - Security vulnerabilities
   - Performance issues
   - Code style and maintainability
   - Missing tests or documentation
3. Present findings categorized by severity (Critical, High, Medium, Low).
4. Provide concrete recommendations with code examples where applicable.
5. Summarize top 3 priorities for improvement.

Be thorough but constructive. Always explain WHY something is an issue.`
}

// BuildMessages constructs messages for the code review flow.
func (f *CodeReviewFlow) BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, metadata map[string]any) []llm.Message {
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

// UpdateState tracks code review flow progress.
func (f *CodeReviewFlow) UpdateState(metadata map[string]any, assistantContent string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}
	metadata["flow"] = f.Name()
	return metadata
}
