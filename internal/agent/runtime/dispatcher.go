package runtime

import (
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

	// Build tool definitions
	var toolDefs []llm.ToolDefinition
	for _, t := range toolList {
		toolDefs = append(toolDefs, llm.ToolDefinition{
			Type: "function",
			Function: llm.FunctionSchema{
				Name:        t.Name(),
				Description: t.Description(),
				Parameters:  t.Schema(),
			},
		})
	}

	return llm.Request{
		Model:       agent.Model,
		Messages:    messages,
		Tools:       toolDefs,
		Temperature: 0.7,
		MaxTokens:   4096,
		Stream:      true,
	}
}
