package runtime

import (
	"fmt"
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// BuildRequest constructs an LLM request for the given agent, history, and user message.
func BuildRequest(agent *model.Agent, history []model.Message, userMessage string, toolList []tools.Tool) llm.Request {
	var messages []llm.Message

	// System prompt
	systemContent := agent.SystemPrompt
	if systemContent == "" {
		systemContent = string(agent.Role)
	}

	// Append available tools to system prompt
	if len(toolList) > 0 {
		var desc strings.Builder
		desc.WriteString("\n\nYou have access to the following tools:\n")
		for _, t := range toolList {
			desc.WriteString(fmt.Sprintf("- %s: %s\n", t.Name(), t.Description()))
		}
		desc.WriteString("\nTo use a tool, respond exactly with:\n" +
			"TOOL_CALL: {\"tool\":\"name\",\"args\":{...}} :END_TOOL_CALL\n" +
			"Otherwise respond normally.")
		systemContent += desc.String()
	}

	messages = append(messages, llm.Message{Role: "system", Content: systemContent})

	// History
	for _, h := range history {
		role := string(h.Role)
		content := h.Content
		if len(h.ToolCalls) > 0 {
			content += "\n[tool calls]"
		}
		messages = append(messages, llm.Message{Role: role, Content: content})
	}

	// User message
	messages = append(messages, llm.Message{Role: "user", Content: userMessage})

	return llm.Request{
		Model:       agent.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4096,
		Stream:      true,
	}
}
