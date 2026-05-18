package runtime

import (
	"fmt"
	"strings"
	"time"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// BuildSystemPrompt composes the final system prompt by combining the agent's
// base system prompt with workspace awareness, user context, and tool discipline.
func BuildSystemPrompt(agent *model.Agent, artifacts []string, userContext string) string {
	var b strings.Builder
	b.WriteString(agent.SystemPrompt)
	if agent.SystemPrompt == "" {
		b.WriteString(string(agent.Role))
	}

	// Workspace awareness
	if len(artifacts) > 0 {
		b.WriteString("\n\nWORKSPACE AWARENESS:\n")
		b.WriteString("Before starting any coding task, check if there are existing files with read_file.\n")
		b.WriteString("If files exist: read them first, maintain consistency with existing patterns, and explain what you're changing and why.\n")
		b.WriteString("Current session files: ")
		b.WriteString(strings.Join(artifacts, ", "))
		b.WriteString("\n")
	}

	// User context
	if userContext != "" {
		b.WriteString("\nUSER CONTEXT:\n")
		b.WriteString(userContext)
		b.WriteString("\n")
	}

	// Tool discipline footer
	b.WriteString(`
TOOL DISCIPLINE:
- Think before every response: "Does this task need a tool?"
- Tasks that create something concrete → file_write
- Tasks that verify logic → code_execute
- Tasks needing current info → web_search
- Pure explanation or conversation → no tool needed
- After using a tool, always tell the user what you did and what they can do next.
- NEVER use web_search for questions about current date, day, or time. Use the TEMPORAL CONTEXT provided in USER CONTEXT.
`)

	return b.String()
}

// BuildMessages constructs the LLM message list from agent, history, user message,
// and optional workspace artifacts.
func BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, artifacts []string) []llm.Message {
	var messages []llm.Message

	systemContent := BuildSystemPrompt(agent, artifacts, userContext)
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

// BuildTemporalContext returns a formatted temporal context string for the given timezone.
// If the timezone is empty or invalid, it falls back to UTC.
func BuildTemporalContext(timezone string) string {
	if timezone == "" {
		timezone = "UTC"
	}
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	return fmt.Sprintf(
		"TEMPORAL CONTEXT:\n"+
			"Current date: %s\n"+
			"Current day: %s\n"+
			"Current time: %s\n"+
			"Timezone: %s\n",
		now.Format("2006-01-02"),
		now.Weekday().String(),
		now.Format("15:04:05"),
		timezone,
	)
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
