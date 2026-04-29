package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/prompts"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/document"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Orchestrator routes messages through classification → LLM → tools → response.
type Orchestrator struct {
	llmClient     llm.Client
	registry      *tools.Registry
	sessions      *SessionManager
	agentRepo     repository.AgentRepo
	fileSvc       *fileupload.Service
	docClient     *document.Client
	classifier    *LLMClassifier
}

// NewOrchestrator creates a new chat orchestrator.
func NewOrchestrator(client llm.Client, registry *tools.Registry, sessions *SessionManager, agentRepo repository.AgentRepo, fileSvc *fileupload.Service, docClient *document.Client) *Orchestrator {
	return &Orchestrator{
		llmClient:  client,
		registry:   registry,
		sessions:   sessions,
		agentRepo:  agentRepo,
		fileSvc:    fileSvc,
		docClient:  docClient,
		classifier: NewLLMClassifier(client),
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

	// 4. Classify intent with LLM, fallback to keyword matching
	var role model.AgentRole
	if o.classifier != nil {
		classifiedRole, err := o.classifier.Classify(ctx, content)
		if err != nil {
			role = ClassifyKeyword(content)
		} else {
			role = classifiedRole
		}
	} else {
		role = ClassifyKeyword(content)
	}

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
		var toolCall *llm.ToolCall
		for chunk := range llmCh {
			assistantContent.WriteString(chunk.Content)
			if chunk.ToolCall != nil {
				toolCall = chunk.ToolCall
			}
			select {
			case outCh <- chunk:
			case <-ctx.Done():
				return
			}
			if chunk.Done {
				break
			}
		}

		// 9. Execute native tool call if present
		if toolCall != nil {
			// Inject tool context
			toolCtx := context.WithValue(ctx, "tenant_id", tenantID)
			toolCtx = context.WithValue(toolCtx, "conversation_id", conversationID)
			res, err := o.registry.Execute(toolCtx, toolCall.Name, toolCall.Arguments)
			if err != nil {
				res.Error = err.Error()
			}
			resultChunk := llm.Chunk{
				Content: fmt.Sprintf("\n[Tool %s result: %s]", toolCall.Name, res.Content),
				Done:    true,
			}
			if res.Error != "" {
				resultChunk.Content = fmt.Sprintf("\n[Tool %s error: %s]", toolCall.Name, res.Error)
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
		if toolCall != nil {
			assistantMsg.ToolCalls = []model.ToolCall{{
				ID:        uuid.New().String(),
				Name:      toolCall.Name,
				Arguments: toolCall.Arguments,
			}}
		}
		_ = o.sessions.AddMessage(ctx, assistantMsg)
	}()

	return outCh, nil
}

func (o *Orchestrator) resolveAgent(ctx context.Context, tenantID uuid.UUID, role model.AgentRole) (*model.Agent, error) {
	// Try to get specialized agent from DB
	agent, err := o.agentRepo.GetByRole(ctx, tenantID, role)
	if err == nil && agent != nil {
		return agent, nil
	}

	// Fallback to default agent
	return o.createDefaultAgent(role), nil
}

func (o *Orchestrator) createDefaultAgent(role model.AgentRole) *model.Agent {
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
	return agent
}

func (o *Orchestrator) buildFileContext(ctx context.Context, tenantID uuid.UUID, fileIDs []uuid.UUID) (string, error) {
	var parts []string
	for _, id := range fileIDs {
		upload, err := o.fileSvc.Get(ctx, tenantID, id)
		if err != nil {
			continue
		}
		
		var text string
		
		// Try to read as text first
		text, err = o.fileSvc.ReadText(upload)
		if err != nil {
			// Not a text file - try document extraction service
			if o.docClient != nil {
				file, err := o.fileSvc.Open(upload)
				if err != nil {
					continue
				}
				defer file.Close()
				
				var result *document.ExtractResponse
				switch upload.ContentType {
				case "application/pdf":
					result, err = o.docClient.ExtractPDF(ctx, upload.OriginalName, file)
				case "application/vnd.openxmlformats-officedocument.wordprocessingml.document":
					result, err = o.docClient.ExtractDOCX(ctx, upload.OriginalName, file)
				case "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":
					result, err = o.docClient.ExtractXLSX(ctx, upload.OriginalName, file)
				case "image/png", "image/jpeg", "image/jpg", "image/webp", "image/gif":
					result, err = o.docClient.ExtractImage(ctx, upload.OriginalName, upload.ContentType, file)
				default:
					continue
				}
				
				if err != nil {
					parts = append(parts, fmt.Sprintf("--- File: %s ---\n[Error extracting content: %v]", upload.OriginalName, err))
					continue
				}
				text = result.Text
			} else {
				continue
			}
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
