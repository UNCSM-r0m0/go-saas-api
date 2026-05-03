package runtime

import (
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

func BuildMessages(agent *model.Agent, history []model.Message, userMessage string) []llm.Message {
	var messages []llm.Message

	systemContent := agent.SystemPrompt
	if systemContent == "" {
		systemContent = string(agent.Role)
	}
	messages = append(messages, llm.Message{Role: "system", Content: systemContent})

	for _, h := range history {
		switch h.Role {
		case model.MessageRoleUser, model.MessageRoleSystem:
			messages = append(messages, llm.Message{Role: string(h.Role), Content: h.Content})
		case model.MessageRoleAssistant:
			if len(h.ToolCalls) > 0 {
				toolCalls := make([]llm.ToolCall, len(h.ToolCalls))
				for i, tc := range h.ToolCalls {
					toolCalls[i] = llm.ToolCall{ID: tc.ID, Name: tc.Name, Arguments: tc.Arguments}
				}
				messages = append(messages, llm.Message{
					Role:      "assistant",
					Content:   h.Content,
					ToolCalls: toolCalls,
				})
			} else {
				messages = append(messages, llm.Message{Role: "assistant", Content: h.Content})
			}
		case model.MessageRoleTool:
			messages = append(messages, llm.Message{
				Role:       "tool",
				Content:    h.Content,
				ToolCallID: h.ToolCallID,
				Name:       h.ToolName,
			})
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: userMessage})
	return messages
}

func BuildToolDefinitions(toolList []tools.Tool) []llm.ToolDefinition {
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
	return toolDefs
}