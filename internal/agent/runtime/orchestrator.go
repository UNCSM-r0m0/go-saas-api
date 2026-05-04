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
	"github.com/r0lm0/go-saas-api/internal/provider"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

type Orchestrator struct {
	llmClient    llm.Client
	registry     *tools.Registry
	sessions     *SessionManager
	agentRepo    repository.AgentRepo
	artRepo      repository.ArtifactRepo
	artFileRepo  repository.ArtifactFileRepo
	fileSvc      *fileupload.Service
	docClient    *document.Client
	providers    provider.Store
	classifier   *LLMClassifier
	log          logger.Logger
}

func NewOrchestrator(client llm.Client, registry *tools.Registry, sessions *SessionManager, agentRepo repository.AgentRepo, artRepo repository.ArtifactRepo, artFileRepo repository.ArtifactFileRepo, fileSvc *fileupload.Service, docClient *document.Client, providers provider.Store, log logger.Logger) *Orchestrator {
	return &Orchestrator{
		llmClient:    client,
		registry:     registry,
		sessions:     sessions,
		agentRepo:    agentRepo,
		artRepo:      artRepo,
		artFileRepo:  artFileRepo,
		fileSvc:      fileSvc,
		docClient:    docClient,
		providers:    providers,
		classifier:   NewLLMClassifier(client),
		log:          log,
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
		capability, err := o.resolveWebsiteAgentCapability(ctx, agent.Model)
		if err != nil {
			return nil, err
		}

		agent.SystemPrompt = prompts.WebsiteAgent()
		agent.Role = model.RoleWebsiteAgent

		messages := BuildMessages(agent, history, userContent, userContext)

		outCh := make(chan llm.Chunk)
		go func() {
			defer close(outCh)
			o.websiteAgentLoop(ctx, outCh, agent, messages, conversationID, capability.MaxTokens)
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

type websiteAgentCapability struct {
	MaxTokens int
}

func (o *Orchestrator) resolveWebsiteAgentCapability(ctx context.Context, modelName string) (websiteAgentCapability, error) {
	const fallbackMaxTokens = 12000
	if o.providers == nil {
		return websiteAgentCapability{MaxTokens: fallbackMaxTokens}, nil
	}

	models, err := o.providers.ListActiveModels(ctx, false)
	if err != nil {
		return websiteAgentCapability{}, fmt.Errorf("validar modelo agentic: %w", err)
	}
	for _, m := range models {
		if m.Name != modelName {
			continue
		}
		if !m.SupportsWebsiteAgent() {
			return websiteAgentCapability{}, fmt.Errorf("el modelo %q no está habilitado para Website Agent", modelName)
		}
		return websiteAgentCapability{MaxTokens: m.WebsiteAgentMaxTokens()}, nil
	}
	return websiteAgentCapability{}, fmt.Errorf("el modelo %q no está activo o no existe", modelName)
}

var htmlBlockRegex = regexp.MustCompile("(?is)```(?:html|markup)\\s*(.*?)(?:```|\\z)")
var doctypeRegex = regexp.MustCompile("(?is)(<!DOCTYPE\\s+html\\b.*?</html>|<!DOCTYPE\\s+html\\b.*\\z)")
var htmlTagRegex = regexp.MustCompile("(?is)(<html\\b.*?</html>|<html\\b.*\\z)")

// multiFileRegex matches the === FILE: path === delimiter format
var multiFileRegex = regexp.MustCompile("(?m)^===\\s*FILE:\\s*(.+?)\\s*===(.*?)(?=^===\\s*FILE:|^===\\s*END\\s*===|\\z)")

// parseMultiFile extracts files from the LLM output using the === FILE: path === format
func parseMultiFile(content string) []model.ArtifactFile {
	matches := multiFileRegex.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var files []model.ArtifactFile
	for i, match := range matches {
		if len(match) < 3 {
			continue
		}
		path := strings.TrimSpace(match[1])
		fileContent := strings.TrimSpace(match[2])
		if path == "" || fileContent == "" {
			continue
		}

		// Detect language from extension
		lang := detectLanguageFromPath(path)

		files = append(files, model.ArtifactFile{
			ID:        uuid.New(),
			Path:      path,
			Language:  lang,
			Content:   fileContent,
			FileOrder: i,
		})
	}

	return files
}

func detectLanguageFromPath(path string) string {
	ext := ""
	if idx := strings.LastIndex(path, "."); idx != -1 {
		ext = path[idx+1:]
	}

	langMap := map[string]string{
		"html": "html", "htm": "html",
		"css": "css", "scss": "scss", "sass": "sass",
		"js": "javascript", "jsx": "jsx",
		"ts": "typescript", "tsx": "tsx",
		"json": "json",
		"md": "markdown",
		"py": "python",
		"go": "go",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "text"
}

const websiteAgentTimeout = 180 * time.Second // 3 minutos para landing pages completas
const firstChunkTimeout = 45 * time.Second
const maxWebsiteContinuations = 2

// websiteAgentLoop streams one or more LLM responses without tools, then extracts a website artifact.
func (o *Orchestrator) websiteAgentLoop(
	ctx context.Context,
	outCh chan<- llm.Chunk,
	agent *model.Agent,
	messages []llm.Message,
	conversationID uuid.UUID,
	maxTokens int,
) {
	select {
	case outCh <- llm.Chunk{Event: "progress", Content: "Generando sitio web..."}:
	case <-ctx.Done():
		return
	}

	var contentBuilder strings.Builder
	attemptMessages := append([]llm.Message(nil), messages...)
	totalChunks := 0
	maxAttempts := 1 + maxWebsiteContinuations

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case outCh <- llm.Chunk{Event: "progress", Content: fmt.Sprintf("Continuando HTML (%d/%d)...", attempt-1, maxWebsiteContinuations)}:
			case <-ctx.Done():
				return
			}
		}

		chunks, gotFirstChunk, err := o.streamWebsiteAgentAttempt(ctx, outCh, &contentBuilder, agent, attemptMessages, conversationID, maxTokens, attempt)
		totalChunks += chunks
		if err != nil {
			o.log.Error("website_agent: stream attempt failed", logger.Error(err), logger.Int("attempt", attempt))
			select {
			case outCh <- llm.Chunk{Event: "error", Content: fmt.Sprintf("Error generando sitio web: %v", err)}:
			case <-ctx.Done():
			}
			return
		}
		if !gotFirstChunk {
			o.log.Warn("website_agent: stream closed without chunks", logger.Int("attempt", attempt))
			select {
			case outCh <- llm.Chunk{Event: "error", Content: "El agente no devolvió contenido. Reintentá con una instrucción más corta."}:
			case <-ctx.Done():
			}
			return
		}

		fullContent := contentBuilder.String()
		htmlContent, _ := extractWebsiteHTML(fullContent)
		if htmlContent != "" && !needsWebsiteContinuation(htmlContent) {
			break
		}
		if attempt == maxAttempts {
			break
		}

		attemptMessages = continuationMessages(messages, fullContent)
	}

	fullContent := contentBuilder.String()
	o.log.Info("website_agent: stream complete",
		logger.Int("chunks", totalChunks),
		logger.Int("content_length", len(fullContent)))

	select {
	case outCh <- llm.Chunk{Event: "progress", Content: "Extrayendo archivos..."}:
	case <-ctx.Done():
		o.log.Warn("website_agent: context cancelled before extraction")
		return
	}

	// Try multi-file format first
	files := parseMultiFile(fullContent)
	var htmlContent string
	var artifactID string

	if len(files) > 0 {
		o.log.Info("website_agent: multi-file project detected",
			logger.Int("file_count", len(files)),
			logger.String("files", strings.Join(getFilePaths(files), ", ")))

		// Find entry HTML file
		for _, f := range files {
			if f.Path == "index.html" {
				htmlContent = f.Content
				break
			}
		}

		artifactID = o.saveWebsiteArtifact(ctx, conversationID, htmlContent, files)
	} else {
		// Fallback to single HTML extraction
		htmlContent, matchType := extractWebsiteHTML(fullContent)
		if htmlContent != "" {
			o.log.Info("website_agent: HTML extracted",
				logger.String("match_type", matchType),
				logger.Int("html_length", len(htmlContent)),
				logger.String("html_preview", truncate(htmlContent, 80)))
		} else {
			o.log.Warn("website_agent: no HTML pattern matched", logger.String("preview", truncate(fullContent, 200)))
		}

		artifactID = o.saveWebsiteArtifact(ctx, conversationID, htmlContent, nil)
	}

	o.saveWebsiteAssistantMessage(ctx, conversationID, agent.Model, fullContent, artifactID)
	o.sendWebsiteArtifactResult(ctx, outCh, artifactID)
}

func (o *Orchestrator) streamWebsiteAgentAttempt(
	ctx context.Context,
	outCh chan<- llm.Chunk,
	contentBuilder *strings.Builder,
	agent *model.Agent,
	messages []llm.Message,
	conversationID uuid.UUID,
	maxTokens int,
	attempt int,
) (int, bool, error) {
	req := llm.Request{
		Model:       agent.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   maxTokens,
		Stream:      true,
	}

	o.log.Info("website_agent: calling LLM Stream",
		logger.String("model", agent.Model),
		logger.Int("msg_count", len(messages)),
		logger.Int("max_tokens", maxTokens),
		logger.Int("attempt", attempt),
		logger.String("conversation_id", conversationID.String()))

	llmCtx, llmCancel := context.WithTimeout(ctx, websiteAgentTimeout)
	defer llmCancel()

	llmCh, err := o.llmClient.Stream(llmCtx, req)
	if err != nil {
		return 0, false, err
	}

	chunkCount := 0
	firstChunkReceived := false
	for chunk := range llmCh {
		if !firstChunkReceived {
			firstChunkReceived = true
			o.log.Info("website_agent: received first chunk",
				logger.Int("attempt", attempt),
				logger.String("content_preview", truncate(chunk.Content, 50)))
		}
		chunkCount++

		if chunk.Content == "" {
			continue
		}
		contentBuilder.WriteString(chunk.Content)
		select {
		case outCh <- llm.Chunk{Content: chunk.Content}:
		case <-ctx.Done():
			o.log.Warn("website_agent: context cancelled during streaming")
			return chunkCount, firstChunkReceived, ctx.Err()
		}
	}

	return chunkCount, firstChunkReceived, nil
}

func continuationMessages(base []llm.Message, fullContent string) []llm.Message {
	messages := append([]llm.Message(nil), base...)
	messages = append(messages,
		llm.Message{Role: "assistant", Content: fullContent},
		llm.Message{Role: "user", Content: `Continuá exactamente desde donde quedaste. No repitas lo anterior. Devolvé SOLO la continuación faltante para terminar el mismo documento HTML. Cerrá correctamente cualquier CSS, JS, body y html abierto.`},
	)
	return messages
}

func extractWebsiteHTML(content string) (string, string) {
	if match := htmlBlockRegex.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1]), "markdown_code_block"
	}
	if match := doctypeRegex.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1]), "doctype_html"
	}
	if match := htmlTagRegex.FindStringSubmatch(content); len(match) > 1 {
		return strings.TrimSpace(match[1]), "html_tag"
	}
	return "", ""
}

func needsWebsiteContinuation(html string) bool {
	lower := strings.ToLower(html)
	return strings.Contains(lower, "<html") && !strings.Contains(lower, "</html>") ||
		strings.Contains(lower, "<style") && !strings.Contains(lower, "</style>") ||
		strings.Contains(lower, "<script") && !strings.Contains(lower, "</script>") ||
		strings.Contains(lower, "<body") && !strings.Contains(lower, "</body>")
}

func (o *Orchestrator) saveWebsiteArtifact(ctx context.Context, conversationID uuid.UUID, htmlContent string, files []model.ArtifactFile) string {
	if htmlContent == "" && len(files) == 0 {
		return ""
	}

	art := &model.Artifact{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Name:           "Landing Page",
		Type:           "website",
		Language:       "typescript",
		Content:        htmlContent,
		EntryFile:      "index.html",
		IsComplete:     true,
		Version:        1,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if o.artRepo == nil {
		o.log.Warn("website_agent: artRepo is nil, artifact not saved")
		return ""
	}

	// Save main artifact
	if err := o.artRepo.Create(ctx, art); err != nil {
		o.log.Error("website_agent: failed to save artifact", logger.Error(err), logger.String("conversation_id", conversationID.String()))
		return ""
	}

	// Save individual files if multi-file project
	if len(files) > 0 && o.artFileRepo != nil {
		for i := range files {
			files[i].ArtifactID = art.ID
		}
		if err := o.artFileRepo.CreateMany(ctx, files); err != nil {
			o.log.Error("website_agent: failed to save artifact files", logger.Error(err), logger.String("artifact_id", art.ID.String()))
			return ""
		}
		o.log.Info("website_agent: multi-file artifact saved",
			logger.String("artifact_id", art.ID.String()),
			logger.Int("file_count", len(files)))
	} else {
		o.log.Info("website_agent: single-file artifact saved",
			logger.String("artifact_id", art.ID.String()),
			logger.Int("html_length", len(htmlContent)))
	}

	return art.ID.String()
}

func (o *Orchestrator) saveWebsiteAssistantMessage(ctx context.Context, conversationID uuid.UUID, modelName, content, artifactID string) {
	msg := &model.Message{
		ID:             uuid.New(),
		ConversationID: conversationID,
		Role:           model.MessageRoleAssistant,
		Content:        content,
		Model:          modelName,
		CreatedAt:      time.Now(),
	}
	if artifactID != "" {
		if artUUID, err := uuid.Parse(artifactID); err == nil {
			msg.ArtifactID = &artUUID
		}
	}
	if err := o.sessions.AddMessage(ctx, msg); err != nil {
		o.log.Error("website_agent: failed to save assistant message", logger.Error(err))
		return
	}
	o.log.Info("website_agent: assistant message saved", logger.String("message_id", msg.ID.String()), logger.Int("content_length", len(content)), logger.String("artifact_id", artifactID))
}

func (o *Orchestrator) sendWebsiteArtifactResult(ctx context.Context, outCh chan<- llm.Chunk, artifactID string) {
	if artifactID != "" {
		select {
		case outCh <- llm.Chunk{Event: "artifact", Content: artifactID}:
		case <-ctx.Done():
			return
		}
	} else {
		select {
		case outCh <- llm.Chunk{Event: "error", Content: "No se encontró HTML utilizable en la respuesta. Intentá con una descripción más corta o usá un modelo habilitado para Website Agent."}:
		case <-ctx.Done():
			return
		}
	}
	select {
	case outCh <- llm.Chunk{Done: true}:
	case <-ctx.Done():
	}
}

func getFilePaths(files []model.ArtifactFile) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}
