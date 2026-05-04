package runtime

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/prompts"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/document"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

type Orchestrator struct {
	llmClient  llm.Client
	registry   *tools.Registry
	sessions   *SessionManager
	agentRepo  repository.AgentRepo
	artRepo    repository.ArtifactRepo
	fileSvc    *fileupload.Service
	docClient  *document.Client
	classifier *LLMClassifier
	log        logger.Logger
}

func NewOrchestrator(client llm.Client, registry *tools.Registry, sessions *SessionManager, agentRepo repository.AgentRepo, artRepo repository.ArtifactRepo, fileSvc *fileupload.Service, docClient *document.Client, log logger.Logger) *Orchestrator {
	return &Orchestrator{
		llmClient:  client,
		registry:   registry,
		sessions:   sessions,
		agentRepo:  agentRepo,
		artRepo:    artRepo,
		fileSvc:    fileSvc,
		docClient:  docClient,
		classifier: NewLLMClassifier(client),
		log:        log,
	}
}

func (o *Orchestrator) Chat(ctx context.Context, userID uuid.UUID, convID *uuid.UUID, content string, fileIDs []uuid.UUID, selectedModel string, userContext string, mode string) (<-chan llm.Chunk, error) {
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

	// Website agent mode: bypass tools, use special system prompt, extract HTML artifact
	if mode == "website_agent" {
		agent.SystemPrompt = prompts.WebsiteAgent()
		agent.Role = model.RoleWebsiteAgent

		messages := BuildMessages(agent, history, userContent, userContext)

		outCh := make(chan llm.Chunk)
		go func() {
			defer close(outCh)
			o.websiteAgentLoop(ctx, outCh, agent, messages, conversationID)
		}()
		return outCh, nil
	}

	toolList := o.registry.List()
	var toolInstances []tools.Tool
	for _, name := range toolList {
		if t, ok := o.registry.Get(name); ok {
			toolInstances = append(toolInstances, t)
		}
	}

	messages := BuildMessages(agent, history, userContent, userContext)
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

var htmlBlockRegex = regexp.MustCompile("(?s)```html\\s*(.*?)\\s*```")
var doctypeRegex = regexp.MustCompile("(?s)(<!DOCTYPE html>.*?</html>)")

const websiteAgentTimeout = 90 * time.Second
const firstChunkTimeout = 30 * time.Second

// websiteAgentLoop streams a single LLM response without tools, extracts HTML artifact if present.
func (o *Orchestrator) websiteAgentLoop(
	ctx context.Context,
	outCh chan<- llm.Chunk,
	agent *model.Agent,
	messages []llm.Message,
	conversationID uuid.UUID,
) {
	// Send progress event immediately
	select {
	case outCh <- llm.Chunk{Event: "progress", Content: "Generando sitio web..."}:
	case <-ctx.Done():
		return
	}

	req := llm.Request{
		Model:       agent.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4096,
		Stream:      true,
	}

	o.log.Info("website_agent: calling LLM Stream", logger.String("model", agent.Model), logger.Int("msg_count", len(messages)), logger.String("conversation_id", conversationID.String()))

	// Create a timeout context for the LLM call
	llmCtx, llmCancel := context.WithTimeout(ctx, websiteAgentTimeout)
	defer llmCancel()

	llmCh, err := o.llmClient.Stream(llmCtx, req)
	if err != nil {
		o.log.Error("website_agent: LLM stream failed", logger.Error(err))
		select {
		case outCh <- llm.Chunk{Event: "error", Content: fmt.Sprintf("Error iniciando generación: %v", err), Done: true}:
		case <-ctx.Done():
		}
		return
	}

	o.log.Info("website_agent: LLM stream started successfully")

	var contentBuilder strings.Builder
	chunkCount := 0
	firstChunkReceived := false

	for chunk := range llmCh {
		if !firstChunkReceived {
			firstChunkReceived = true
			o.log.Info("website_agent: received first chunk", logger.String("content_preview", truncate(chunk.Content, 50)))
		}
		chunkCount++

		if chunk.Content != "" {
			contentBuilder.WriteString(chunk.Content)
			select {
			case outCh <- llm.Chunk{Content: chunk.Content}:
			case <-ctx.Done():
				return
			}
		}
		if chunk.Done {
			break
		}
	}

	if !firstChunkReceived {
		o.log.Warn("website_agent: stream closed without any chunks")
		select {
		case outCh <- llm.Chunk{Event: "error", Content: "El agente tardó demasiado en responder. Reintentá con una instrucción más corta.", Done: true}:
		case <-ctx.Done():
		}
		return
	}

	o.log.Info("website_agent: stream complete", logger.Int("chunks", chunkCount), logger.Int("content_length", contentBuilder.Len()))

	// Send progress event
	select {
	case outCh <- llm.Chunk{Event: "progress", Content: "Extrayendo HTML..."}:
	case <-ctx.Done():
		return
	}

	fullContent := contentBuilder.String()

	// Extract HTML artifact - try multiple patterns
	var artifactID string
	var htmlContent string

	// Try markdown code block first
	if match := htmlBlockRegex.FindStringSubmatch(fullContent); len(match) > 1 {
		htmlContent = strings.TrimSpace(match[1])
		o.log.Info("website_agent: found HTML in markdown block")
	} else if match := doctypeRegex.FindStringSubmatch(fullContent); len(match) > 1 {
		// Fallback: extract raw HTML document
		htmlContent = strings.TrimSpace(match[1])
		o.log.Info("website_agent: found raw HTML document")
	}

	if htmlContent != "" {
		art := &model.Artifact{
			ID:             uuid.New(),
			ConversationID: conversationID,
			Name:           "index.html",
			Type:           "website",
			Language:       "html",
			Content:        htmlContent,
			Version:        1,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if o.artRepo != nil {
			if err := o.artRepo.Create(ctx, art); err != nil {
				o.log.Error("website_agent: failed to save artifact", logger.Error(err))
			} else {
				artifactID = art.ID.String()
				o.log.Info("website_agent: artifact saved", logger.String("artifact_id", artifactID))
			}
		}
	} else {
		o.log.Warn("website_agent: no HTML found in response")
	}

	// Save assistant message
	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Role:           model.MessageRoleAssistant,
		Content:        fullContent,
		Model:          agent.Model,
		CreatedAt:      time.Now(),
	}
	if artifactID != "" {
		if artUUID, err := uuid.Parse(artifactID); err == nil {
			msg.ArtifactID = &artUUID
		}
	}
	_ = o.sessions.AddMessage(ctx, msg)

	// Send artifact metadata as final chunk
	if artifactID != "" {
		select {
		case outCh <- llm.Chunk{Event: "artifact", Content: artifactID, Done: false}:
		case <-ctx.Done():
			return
		}
	}

	select {
	case outCh <- llm.Chunk{Done: true}:
	case <-ctx.Done():
	}
}
