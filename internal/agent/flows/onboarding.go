package flows

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// OnboardingFlow guides new users through a 3-question onboarding.
type OnboardingFlow struct{}

// NewOnboardingFlow creates a new onboarding flow.
func NewOnboardingFlow() *OnboardingFlow {
	return &OnboardingFlow{}
}

// Name returns the flow identifier.
func (f *OnboardingFlow) Name() string { return "onboarding" }

// Detect triggers on the first conversation (historyLen == 0) or explicit onboarding intent.
func (f *OnboardingFlow) Detect(message string, historyLen int) (bool, float64) {
	if historyLen == 0 {
		return true, 0.95
	}
	lower := strings.ToLower(message)
	if strings.Contains(lower, "onboard") || strings.Contains(lower, "empezar") || strings.Contains(lower, "configurar") {
		return true, 0.85
	}
	return false, 0
}

// SystemPrompt returns the onboarding system prompt.
func (f *OnboardingFlow) SystemPrompt() string {
	return `You are onboarding a new user. Your goal is to understand their needs in exactly 3 questions.

FLOW RULES (follow STRICTLY):
1. Ask question 1: "What's your main use case? (coding, writing, research, business automation)"
2. Wait for the user's answer. Then ask ONE relevant follow-up based on their answer (e.g., tech stack, industry, team size).
3. Wait for the user's answer. Then ask: "What's one thing that frustrates you about current AI tools?"
4. After the 3rd answer, summarize what you learned and tell them how you'll use it to help them.
5. NEVER ask more than 3 questions. NEVER repeat a question.

EXTRACTION:
At the end, you MUST extract these fields and present them clearly:
- preferred_use_case
- technical_level
- pain_points

This is the ONLY conversation where you ask onboarding questions. Be friendly and concise.`
}

// BuildMessages constructs messages with flow state tracking.
func (f *OnboardingFlow) BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, metadata map[string]any) []llm.Message {
	var messages []llm.Message

	systemContent := f.SystemPrompt()
	if agent.SystemPrompt != "" {
		systemContent = agent.SystemPrompt + "\n\n" + systemContent
	}
	if userContext != "" {
		systemContent += "\n" + userContext
	}

	// Inject current flow step into system prompt
	step := f.getStep(metadata)
	if step > 0 {
		systemContent += fmt.Sprintf("\n\n[FLOW STATE] Current step: %d/3. Do NOT repeat previous questions.", step)
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

// UpdateState tracks the onboarding step from the conversation.
func (f *OnboardingFlow) UpdateState(metadata map[string]any, assistantContent string) map[string]any {
	if metadata == nil {
		metadata = make(map[string]any)
	}

	step := f.getStep(metadata)
	// Count questions asked in this response
	questionCount := strings.Count(assistantContent, "?")
	if questionCount > 0 {
		step++
	}
	metadata["flow"] = f.Name()
	metadata["flow_step"] = step
	metadata["flow_completed"] = step >= 3
	return metadata
}

func (f *OnboardingFlow) getStep(metadata map[string]any) int {
	if metadata == nil {
		return 0
	}
	v, ok := metadata["flow_step"]
	if !ok {
		return 0
	}
	switch val := v.(type) {
	case int:
		return val
	case float64:
		return int(val)
	case string:
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return 0
}
