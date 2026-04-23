package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/prompts"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Orchestrator routes messages through classification → LLM → tools → response.
type Orchestrator struct {
	llmClient llm.Client
	registry  *tools.Registry
	sessions  *SessionManager
	agentRepo repository.AgentRepo
	fileSvc   *fileupload.Service
}

// NewOrchestrator creates a new chat orchestrator.
func NewOrchestrator(client llm.Client, registry *tools.Registry, sessions *SessionManager, agentRepo repository.AgentRepo, fileSvc *fileupload.Service) *Orchestrator {
	return &Orchestrator{
		llmClient: client,
		registry:  registry,
		sessions:  sessions,
		agentRepo: agentRepo,
		fileSvc:   fileSvc,
	}
}

// Chat handles a single user turn and returns a stream of chunks.
func (o *Orchestrator) Chat(ctx context.Context, tenantID, userID uuid.UUID, convID *uuid.UUID, content string, fileIDs []uuid.UUID) (<-chan llm.Chunk, error) {
	// 1. Ensure conversation exists
	var conversationID uuid.UUID
	if convID != nil {
		conversationID = *convID
	} else {
		conv, err := o.sessions.CreateConversation(ctx, tenantID, userID, content[:min(50, len(content))], nil)
		if err != nil {
			return nil, fmt.Errorf("create conversation: %w", err)
		}
		conversationID = conv.ID
	}

	// 2. Save user message
	userMsg := &model.Message{
		ID:             uuid.New(),
		TenantID:       tenantID,
		ConversationID: conversationID,
		Role:           model.MessageRoleUser,
		Content:        content,
		CreatedAt:      time.Now(),
	}
	if err := o.sessions.AddMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// 3. Load history
	history, err := o.sessions.GetHistory(ctx, tenantID, conversationID, 50)
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}

	// 4. Classify intent
	role := Classify(content)

	// 5. Resolve agent
	agent, err := o.resolveAgent(ctx, tenantID, role)
	if err != nil {
		return nil, fmt.Errorf("resolve agent: %w", err)
	}

	// 6. Attach file contents if provided
	if len(fileIDs) > 0 && o.fileSvc != nil {
		fileContext, err := o.buildFileContext(ctx, tenantID, fileIDs)
		if err == nil && fileContext != "" {
			content = fileContext + "\n\n" + content
		}
	}

	// 7. Build request with tools
	toolList := o.registry.List()
	var toolInstances []tools.Tool
	for _, name := range toolList {
		if t, ok := o.registry.Get(name); ok {
			toolInstances = append(toolInstances, t)
		}
	}
	req := BuildRequest(agent, history, content, toolInstances)

	// 7. Start LLM stream
	llmCh, err := o.llmClient.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("start llm stream: %w", err)
	}

	// 8. Process stream in goroutine
	outCh := make(chan llm.Chunk)
	go func() {
		defer close(outCh)
		var assistantContent strings.Builder
		for chunk := range llmCh {
			assistantContent.WriteString(chunk.Content)
			select {
			case outCh <- chunk:
			case <-ctx.Done():
				return
			}
			if chunk.Done {
				break
			}
		}

		// 9. Parse tool calls from accumulated content
		toolResult, hasTool := o.parseToolCall(assistantContent.String())
		if hasTool {
			// Inject tool context
			toolCtx := context.WithValue(ctx, "tenant_id", tenantID)
			toolCtx = context.WithValue(toolCtx, "conversation_id", conversationID)
			res, err := o.registry.Execute(toolCtx, toolResult.Name, toolResult.Arguments)
			if err != nil {
				res.Error = err.Error()
			}
			resultChunk := llm.Chunk{
				Content: fmt.Sprintf("\n[Tool %s result: %s]", toolResult.Name, res.Content),
				Done:    true,
			}
			if res.Error != "" {
				resultChunk.Content = fmt.Sprintf("\n[Tool %s error: %s]", toolResult.Name, res.Error)
			}
			select {
			case outCh <- resultChunk:
			case <-ctx.Done():
				return
			}
			assistantContent.WriteString(resultChunk.Content)
		}

		// 10. Save assistant message
		assistantMsg := &model.Message{
			ID:             uuid.New(),
			TenantID:       tenantID,
			ConversationID: conversationID,
			Role:           model.MessageRoleAssistant,
			Content:        assistantContent.String(),
			Model:          agent.Model,
			CreatedAt:      time.Now(),
		}
		if hasTool {
			assistantMsg.ToolCalls = []model.ToolCall{{
				ID:        uuid.New().String(),
				Name:      toolResult.Name,
				Arguments: toolResult.Arguments,
			}}
		}
		_ = o.sessions.AddMessage(ctx, assistantMsg)
	}()

	return outCh, nil
}

func (o *Orchestrator) resolveAgent(ctx context.Context, tenantID uuid.UUID, role model.AgentRole) (*model.Agent, error) {
	// TODO: lookup specialized agent by role from agentRepo
	// For now, return a default agent based on role
	agent := &model.Agent{
		ID:    uuid.New(),
		Name:  string(role),
		Role:  role,
		Model: "qwen2.5-coder:7b",
	}
	switch role {
	case model.RoleCoder:
		agent.SystemPrompt = prompts.Coder()
	case model.RoleResearcher:
		agent.SystemPrompt = prompts.Conversational() + "\nYou are a thorough researcher."
	case model.RoleCopywriter:
		agent.SystemPrompt = prompts.Conversational() + "\nYou are a creative copywriter."
	default:
		agent.SystemPrompt = prompts.Conversational()
	}
	return agent, nil
}

type inlineToolCall struct {
	Name      string         `json:"tool"`
	Arguments map[string]any `json:"args"`
}

func (o *Orchestrator) parseToolCall(content string) (*inlineToolCall, bool) {
	startMarker := "TOOL_CALL:"
	endMarker := ":END_TOOL_CALL"
	start := strings.Index(content, startMarker)
	end := strings.Index(content, endMarker)
	if start == -1 || end == -1 || end <= start {
		return nil, false
	}
	jsonStr := strings.TrimSpace(content[start+len(startMarker) : end])
	var tc inlineToolCall
	if err := json.Unmarshal([]byte(jsonStr), &tc); err != nil {
		return nil, false
	}
	if tc.Name == "" {
		return nil, false
	}
	_, ok := o.registry.Get(tc.Name)
	return &tc, ok
}

func (o *Orchestrator) buildFileContext(ctx context.Context, tenantID uuid.UUID, fileIDs []uuid.UUID) (string, error) {
	var parts []string
	for _, id := range fileIDs {
		upload, err := o.fileSvc.Get(ctx, tenantID, id)
		if err != nil {
			continue
		}
		text, err := o.fileSvc.ReadText(upload)
		if err != nil {
			continue
		}
		if len(text) > 10000 {
			text = text[:10000] + "\n...[truncated]"
		}
		parts = append(parts, fmt.Sprintf("--- File: %s ---\n%s", upload.OriginalName, text))
	}
	if len(parts) == 0 {
		return "", nil
	}
	return "Attached files:\n" + strings.Join(parts, "\n\n"), nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
