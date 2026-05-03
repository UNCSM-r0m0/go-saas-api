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

type Orchestrator struct {
	llmClient  llm.Client
	registry   *tools.Registry
	sessions   *SessionManager
	agentRepo  repository.AgentRepo
	fileSvc    *fileupload.Service
	docClient  *document.Client
	classifier *LLMClassifier
}

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

func (o *Orchestrator) Chat(ctx context.Context, userID uuid.UUID, convID *uuid.UUID, content string, fileIDs []uuid.UUID, selectedModel string) (<-chan llm.Chunk, error) {
	var conversationID uuid.UUID
	if convID != nil {
		conversationID = *convID
	} else {
		conv, err := o.sessions.CreateConversation(ctx, userID, content[:min(50, len(content))], nil)
		if err != nil {
			return nil, fmt.Errorf("create conversation: %w", err)
		}
		conversationID = conv.ID
	}

	userMsg := &model.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Role:           model.MessageRoleUser,
		Content:        content,
		CreatedAt:      time.Now(),
	}
	if err := o.sessions.AddMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	if convID == nil {
		go o.generateAndSaveTitle(ctx, userID, conversationID, content)
	}

	history, err := o.sessions.GetHistory(ctx, conversationID, 50)
	if err != nil {
		return nil, fmt.Errorf("load history: %w", err)
	}

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

	agent, err := o.resolveAgent(ctx, role)
	if err != nil {
		return nil, fmt.Errorf("resolve agent: %w", err)
	}
	if selectedModel != "" {
		agent.Model = selectedModel
	}

	userContent := content
	if len(fileIDs) > 0 && o.fileSvc != nil {
		fileContext, err := o.buildFileContext(ctx, fileIDs)
		if err == nil && fileContext != "" {
			userContent = fileContext + "\n\n" + content
		}
	}

	toolList := o.registry.List()
	var toolInstances []tools.Tool
	for _, name := range toolList {
		if t, ok := o.registry.Get(name); ok {
			toolInstances = append(toolInstances, t)
		}
	}

	messages := BuildMessages(agent, history, userContent)
	toolDefs := BuildToolDefinitions(toolInstances)

	outCh := make(chan llm.Chunk)
	go func() {
		defer close(outCh)
		o.agentLoop(ctx, outCh, agent, messages, toolDefs, toolInstances, conversationID)
	}()

	return outCh, nil
}

func (o *Orchestrator) resolveAgent(ctx context.Context, role model.AgentRole) (*model.Agent, error) {
	agent, err := o.agentRepo.GetByRole(ctx, role)
	if err == nil && agent != nil {
		return agent, nil
	}
	return o.createDefaultAgent(role), nil
}

func (o *Orchestrator) createDefaultAgent(role model.AgentRole) *model.Agent {
	agent := &model.Agent{
		ID:    uuid.New(),
		Name:  string(role),
		Role:  role,
		Model: "qwen2.5-coder:3b",
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

func (o *Orchestrator) buildFileContext(ctx context.Context, fileIDs []uuid.UUID) (string, error) {
	var parts []string
	for _, id := range fileIDs {
		upload, err := o.fileSvc.Get(ctx, id)
		if err != nil {
			continue
		}

		var text string
		text, err = o.fileSvc.ReadText(upload)
		if err != nil {
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

func (o *Orchestrator) generateAndSaveTitle(ctx context.Context, userID, convID uuid.UUID, firstMessage string) {
	titleCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	prompt := fmt.Sprintf(`Generate a short title (3-6 words, no quotes, no punctuation at end) summarizing this message. Reply ONLY with the title, nothing else.

Message: %s`, truncate(firstMessage, 500))

	req := llm.Request{
		Model:       "qwen2.5-coder:3b",
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		MaxTokens:   20,
		Temperature: 0.3,
	}

	resp, err := o.llmClient.Complete(titleCtx, req)
	if err != nil {
		return
	}

	title := cleanTitle(resp)
	if title == "" {
		return
	}

	_ = o.sessions.UpdateConversationTitle(titleCtx, convID, title)
}

func cleanTitle(raw string) string {
	title := strings.TrimSpace(raw)
	title = strings.Trim(title, `"'`)
	title = strings.TrimRight(title, ".!?")
	title = strings.ReplaceAll(title, "\n", " ")
	if len(title) > 60 {
		title = title[:60]
	}
	return title
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}