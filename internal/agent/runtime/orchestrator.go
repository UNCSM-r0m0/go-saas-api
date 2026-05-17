package runtime

import (
	"context"
	"fmt"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/flows"
	"github.com/r0lm0/go-saas-api/internal/agent/memory"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/prompts"
	"github.com/r0lm0/go-saas-api/internal/agent/quality"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
	"github.com/r0lm0/go-saas-api/internal/agent/tools"
	"github.com/r0lm0/go-saas-api/internal/document"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/internal/platform/logger"
	"github.com/r0lm0/go-saas-api/internal/provider"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

type Orchestrator struct {
	llmClient   llm.Client
	registry    *tools.Registry
	sessions    *SessionManager
	agentRepo   repository.AgentRepo
	artRepo     repository.ArtifactRepo
	artFileRepo repository.ArtifactFileRepo
	fileSvc     *fileupload.Service
	docClient   *document.Client
	providers   provider.Store
	classifier   *LLMClassifier
	extractor    *memory.Extractor
	qualityGate  *quality.Gate
	flowRegistry *flows.Registry
	log          logger.Logger
}

func NewOrchestrator(client llm.Client, registry *tools.Registry, sessions *SessionManager, agentRepo repository.AgentRepo, artRepo repository.ArtifactRepo, artFileRepo repository.ArtifactFileRepo, fileSvc *fileupload.Service, docClient *document.Client, providers provider.Store, extractor *memory.Extractor, log logger.Logger) *Orchestrator {
	flowReg := flows.NewRegistry()
	flowReg.Register(flows.NewOnboardingFlow())
	flowReg.Register(flows.NewProposalFlow())
	flowReg.Register(flows.NewContractFlow())
	flowReg.Register(flows.NewCodeReviewFlow())

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
		extractor:    extractor,
		qualityGate:  quality.NewGate(client, log),
		flowRegistry: flowReg,
		log:          log,
	}
}

// ExtractMemory triggers background memory extraction for a conversation.
func (o *Orchestrator) ExtractMemory(ctx context.Context, userID uuid.UUID, conversationID uuid.UUID) {
	if o.extractor == nil {
		return
	}
	history, err := o.sessions.GetHistory(ctx, conversationID, 50)
	if err != nil {
		o.log.Warn("ExtractMemory: failed to load history", logger.Error(err))
		return
	}
	go o.extractor.ExtractAndSave(ctx, userID, history)
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

	// Load workspace artifacts for this conversation
	var artifactNames []string
	if o.artRepo != nil {
		arts, err := o.artRepo.ListByConversation(ctx, conversationID)
		if err == nil {
			for _, a := range arts {
				if !a.IsDeleted {
					artifactNames = append(artifactNames, a.Name)
				}
			}
		}
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

	// Classify intent
	var classification model.ClassificationResult
	if o.classifier != nil {
		classResult, err := o.classifier.Classify(ctx, content)
		if err != nil {
			classification.Role = ClassifyKeyword(content)
		} else {
			classification = classResult
		}
	} else {
		classification.Role = ClassifyKeyword(content)
	}

	// Detect active flow
	var activeFlow flows.Flow
	if classification.Flow != "" && o.flowRegistry != nil {
		if f, ok := o.flowRegistry.Get(classification.Flow); ok {
			activeFlow = f
		}
	}
	// Also check flow registry detection for history-length-based flows (e.g. onboarding)
	if activeFlow == nil && o.flowRegistry != nil {
		if f, score := o.flowRegistry.DetectAll(content, len(history)); f != nil && score >= 0.85 {
			activeFlow = f
		}
	}

	agent, err := o.resolveAgent(ctx, classification.Role)
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

		messages := BuildMessages(agent, history, userContent, userContext, artifactNames)

		outCh := make(chan llm.Chunk)
		go func() {
			defer close(outCh)
			o.websiteAgentLoop(ctx, outCh, agent, messages, conversationID, capability.MaxTokens, content)
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

	var messages []llm.Message
	if activeFlow != nil {
		messages = activeFlow.BuildMessages(agent, history, userContent, userContext, nil)
	} else {
		messages = BuildMessages(agent, history, userContent, userContext, artifactNames)
	}
	toolDefs := BuildToolDefinitions(toolInstances)

	outCh := make(chan llm.Chunk)
	go func() {
		defer close(outCh)
		o.agentLoop(ctx, outCh, agent, messages, toolDefs, toolInstances, conversationID, userID, userContent)
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
		agent.SystemPrompt = prompts.Researcher()
	case model.RoleCopywriter:
		agent.SystemPrompt = prompts.Conversational() + "\nYou are a creative copywriter."
	case model.RoleArchitect:
		agent.SystemPrompt = prompts.Architect()
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
	const fallbackMaxTokens = 16000
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

// multiFileDelimiterRegex matches the === FILE: path === delimiter
var multiFileDelimiterRegex = regexp.MustCompile("(?m)^===\\s*FILE:\\s*(.+?)\\s*===\\s*$")
var relativeImportRegex = regexp.MustCompile(`(?m)^\s*import\s+(?:.+?\s+from\s+)?['"](\.{1,2}/[^'"]+)['"]`)

// parseMultiFile extracts files from the LLM output using the === FILE: path === format
func parseMultiFile(content string) []model.ArtifactFile {
	// Find all delimiter positions with their paths
	matches := multiFileDelimiterRegex.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return nil
	}

	var files []model.ArtifactFile
	for i := 0; i < len(matches); i++ {
		match := matches[i]
		if len(match) < 4 {
			continue
		}

		// Extract path from capture group
		pathStart := match[2]
		pathEnd := match[3]
		path := strings.TrimSpace(content[pathStart:pathEnd])

		// Extract content: from end of this delimiter to start of next delimiter (or end)
		contentStart := match[1] // end of full match
		contentEnd := len(content)
		if i < len(matches)-1 {
			contentEnd = matches[i+1][0] // start of next match
		}

		fileContent := strings.TrimSpace(content[contentStart:contentEnd])

		// Remove trailing === END === or similar delimiter if present
		if idx := strings.Index(fileContent, "==="); idx != -1 {
			fileContent = strings.TrimSpace(fileContent[:idx])
		}

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

type websiteFileIssue struct {
	Path   string
	Issue  string
	LineNo int
}

func validateWebsiteProjectFiles(files []model.ArtifactFile) []websiteFileIssue {
	existing := make(map[string]bool, len(files))
	for _, f := range files {
		existing[path.Clean(f.Path)] = true
	}

	var issues []websiteFileIssue
	for _, f := range files {
		cleanPath := path.Clean(f.Path)
		if !isCodeFile(cleanPath) {
			continue
		}

		content := strings.TrimSpace(f.Content)
		if content == "" {
			issues = append(issues, websiteFileIssue{Path: cleanPath, Issue: "archivo vacío"})
			continue
		}

		lines := strings.Split(content, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "import {") && strings.Count(trimmed, "{") > strings.Count(trimmed, "}") {
				issues = append(issues, websiteFileIssue{Path: cleanPath, Issue: "import incompleto o truncado", LineNo: i + 1})
				break
			}
			if strings.HasPrefix(trimmed, "import ") &&
				!strings.Contains(trimmed, " from ") &&
				!strings.HasPrefix(trimmed, "import '") &&
				!strings.HasPrefix(trimmed, "import \"") &&
				!strings.HasSuffix(trimmed, ";") {
				issues = append(issues, websiteFileIssue{Path: cleanPath, Issue: "import inválido", LineNo: i + 1})
				break
			}
		}

		if strings.HasSuffix(content, "import") ||
			strings.HasSuffix(content, "transition") ||
			strings.HasSuffix(content, "className") ||
			strings.HasSuffix(content, "=>") ||
			strings.HasSuffix(content, "(") ||
			strings.HasSuffix(content, "{") ||
			strings.HasSuffix(content, "[") ||
			strings.HasSuffix(content, ",") {
			issues = append(issues, websiteFileIssue{Path: cleanPath, Issue: "archivo probablemente truncado"})
			continue
		}

		if unbalanced := firstUnbalancedDelimiter(content); unbalanced != "" {
			issues = append(issues, websiteFileIssue{Path: cleanPath, Issue: "delimitadores sin cerrar: " + unbalanced})
			continue
		}

		for _, specifier := range relativeImportRegex.FindAllStringSubmatch(content, -1) {
			if len(specifier) < 2 {
				continue
			}
			if !relativeImportExists(cleanPath, specifier[1], existing) {
				issues = append(issues, websiteFileIssue{Path: cleanPath, Issue: "import local apunta a archivo inexistente: " + specifier[1]})
				break
			}
		}
	}
	return issues
}

func isCodeFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".tsx") ||
		strings.HasSuffix(filePath, ".ts") ||
		strings.HasSuffix(filePath, ".jsx") ||
		strings.HasSuffix(filePath, ".js")
}

func firstUnbalancedDelimiter(content string) string {
	pairs := map[rune]rune{')': '(', ']': '[', '}': '{'}
	var stack []rune
	var quote rune
	escaped := false

	for _, r := range content {
		if quote != 0 {
			if escaped {
				escaped = false
				continue
			}
			if r == '\\' {
				escaped = true
				continue
			}
			if r == quote {
				quote = 0
			}
			continue
		}

		if r == '\'' || r == '"' || r == '`' {
			quote = r
			continue
		}
		if r == '(' || r == '[' || r == '{' {
			stack = append(stack, r)
			continue
		}
		if expected, ok := pairs[r]; ok {
			if len(stack) == 0 || stack[len(stack)-1] != expected {
				return string(r)
			}
			stack = stack[:len(stack)-1]
		}
	}

	if quote != 0 {
		return "quote"
	}
	if len(stack) > 0 {
		return string(stack[len(stack)-1])
	}
	return ""
}

func relativeImportExists(fromPath, specifier string, existing map[string]bool) bool {
	base := path.Clean(path.Join(path.Dir(fromPath), specifier))
	candidates := []string{
		base,
		base + ".tsx",
		base + ".ts",
		base + ".jsx",
		base + ".js",
		path.Join(base, "index.tsx"),
		path.Join(base, "index.ts"),
		path.Join(base, "index.jsx"),
		path.Join(base, "index.js"),
	}
	for _, candidate := range candidates {
		if existing[path.Clean(candidate)] {
			return true
		}
	}
	return false
}

func formatWebsiteFileIssues(issues []websiteFileIssue) string {
	var b strings.Builder
	for _, issue := range issues {
		if issue.LineNo > 0 {
			fmt.Fprintf(&b, "- %s:%d: %s\n", issue.Path, issue.LineNo, issue.Issue)
		} else {
			fmt.Fprintf(&b, "- %s: %s\n", issue.Path, issue.Issue)
		}
	}
	return b.String()
}

func (o *Orchestrator) repairWebsiteProjectFiles(ctx context.Context, agent *model.Agent, files []model.ArtifactFile, issues []websiteFileIssue, maxTokens int) []model.ArtifactFile {
	if o.llmClient == nil || len(issues) == 0 {
		return files
	}

	invalidPaths := make(map[string]bool, len(issues))
	for _, issue := range issues {
		invalidPaths[path.Clean(issue.Path)] = true
	}

	var prompt strings.Builder
	prompt.WriteString(`Repará SOLO los archivos inválidos de este proyecto React/Vite.

REGLAS OBLIGATORIAS:
- Respondé únicamente con bloques === FILE: path ===.
- Devolvé el contenido COMPLETO de cada archivo reparado.
- No uses markdown, explicaciones ni texto fuera de los bloques.
- No renombres archivos.
- No dejes imports, JSX, arrays, objetos, funciones o strings incompletos.
- Si un import local apunta a un archivo inexistente, eliminá ese import y resolvélo dentro del mismo archivo o usando un archivo existente.
- Todos los imports deben estar cerrados correctamente.
- Mantené React 19 + TypeScript + Tailwind.

Problemas detectados:
`)
	prompt.WriteString(formatWebsiteFileIssues(issues))
	prompt.WriteString("\nArchivos inválidos actuales:\n")

	for _, f := range files {
		cleanPath := path.Clean(f.Path)
		if !invalidPaths[cleanPath] {
			continue
		}
		fmt.Fprintf(&prompt, "\n=== FILE: %s ===\n%s\n", cleanPath, f.Content)
	}
	prompt.WriteString("\n=== END ===")

repairCtx, cancel := context.WithTimeout(ctx, websiteAgentTimeout)
	defer cancel()

	o.log.Info("🔧 website_agent: calling LLM for repair",
		logger.String("model", agent.Model),
		logger.Int("max_tokens", maxTokens),
		logger.Int("invalid_file_count", len(invalidPaths)))

	content, err := o.llmClient.Complete(repairCtx, llm.Request{
		Model:       agent.Model,
		Messages:    []llm.Message{{Role: "system", Content: prompts.WebsiteAgent()}, {Role: "user", Content: prompt.String()}},
		Temperature: 0.2,
		MaxTokens:   maxTokens,
		Stream:      false,
	})
	if err != nil {
		o.log.Error("❌ website_agent: repair request FAILED", logger.Error(err))
		return files
	}

	repairedFiles := parseMultiFile(content)
	if len(repairedFiles) == 0 {
		o.log.Warn("⚠️ website_agent: repair produced no parseable files",
			logger.String("content_preview", truncate(content, 300)),
			logger.Int("content_length", len(content)))
		return files
	}

	o.log.Info("✅ website_agent: repair SUCCESS",
		logger.Int("files_requested", len(invalidPaths)),
		logger.Int("files_repaired", len(repairedFiles)))

	return mergeArtifactFiles(files, repairedFiles)
}

func mergeArtifactFiles(base []model.ArtifactFile, replacements []model.ArtifactFile) []model.ArtifactFile {
	indexByPath := make(map[string]int, len(base))
	for i, f := range base {
		indexByPath[path.Clean(f.Path)] = i
	}

	for _, replacement := range replacements {
		cleanPath := path.Clean(replacement.Path)
		idx, ok := indexByPath[cleanPath]
		if !ok {
			continue
		}

		replacement.Path = cleanPath
		replacement.ID = base[idx].ID
		replacement.ArtifactID = base[idx].ArtifactID
		replacement.FileOrder = base[idx].FileOrder
		if replacement.Language == "" {
			replacement.Language = detectLanguageFromPath(cleanPath)
		}
		base[idx] = replacement
	}

	return base
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
		"md":   "markdown",
		"py":   "python",
		"go":   "go",
	}

	if lang, ok := langMap[ext]; ok {
		return lang
	}
	return "text"
}

const websiteAgentTimeout = 300 * time.Second // 5 minutos para landing pages con Framer Motion + animaciones
const firstChunkTimeout = 45 * time.Second
const maxWebsiteContinuations = 4
const maxWebsiteRepairAttempts = 2

// websiteAgentLoop streams one or more LLM responses without tools, then extracts a website artifact.
func (o *Orchestrator) websiteAgentLoop(
	ctx context.Context,
	outCh chan<- llm.Chunk,
	agent *model.Agent,
	messages []llm.Message,
	conversationID uuid.UUID,
	maxTokens int,
	userContent string,
) {
	select {
	case outCh <- llm.Chunk{Event: "progress", Content: "Generando sitio web..."}:
	case <-ctx.Done():
		return
	}

	o.log.Info("🚀 website_agent: START",
		logger.String("model", agent.Model),
		logger.Int("max_tokens", maxTokens),
		logger.Int("max_attempts", 1+maxWebsiteContinuations),
		logger.String("conversation_id", conversationID.String()))

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
			o.log.Error("❌ website_agent: stream attempt FAILED",
				logger.Error(err),
				logger.Int("attempt", attempt),
				logger.Int("total_chunks_so_far", totalChunks))
			select {
			case outCh <- llm.Chunk{Event: "error", Content: fmt.Sprintf("Error generando sitio web: %v", err)}:
			case <-ctx.Done():
			}
			return
		}
		if !gotFirstChunk {
			o.log.Warn("⚠️ website_agent: stream closed without ANY chunks",
				logger.Int("attempt", attempt))
			select {
			case outCh <- llm.Chunk{Event: "error", Content: "El agente no devolvió contenido. Reintentá con una instrucción más corta."}:
			case <-ctx.Done():
			}
			return
		}

		fullContent := contentBuilder.String()
		htmlContent, _ := extractWebsiteHTML(fullContent)
		needsMore := htmlContent != "" && needsWebsiteContinuation(htmlContent)

		o.log.Info("📝 website_agent: attempt result",
			logger.Int("attempt", attempt),
			logger.Int("chunks_this_attempt", chunks),
			logger.Int("total_content_length", len(fullContent)),
			logger.Bool("needs_continuation", needsMore))

		if !needsMore {
			o.log.Info("✅ website_agent: content complete, stopping",
				logger.Int("attempt", attempt))
			break
		}
		if attempt == maxAttempts {
			o.log.Warn("⚠️ website_agent: max attempts reached, using what we have",
				logger.Int("max_attempts", maxAttempts),
				logger.Int("content_length", len(fullContent)))
			break
		}

		o.log.Info("🔄 website_agent: requesting continuation",
			logger.Int("next_attempt", attempt+1),
			logger.Int("content_length_so_far", len(fullContent)))

		attemptMessages = continuationMessages(messages, fullContent)
	}

	fullContent := contentBuilder.String()
	o.log.Info("📊 website_agent: STREAM COMPLETE",
		logger.Int("total_chunks", totalChunks),
		logger.Int("content_length", len(fullContent)),
		logger.Int("content_length_kb", len(fullContent)/1024))

	select {
	case outCh <- llm.Chunk{Event: "progress", Content: "Extrayendo archivos..."}:
	case <-ctx.Done():
		o.log.Warn("website_agent: context cancelled before extraction")
		return
	}

	// Try multi-file format first
	files := parseMultiFile(fullContent)

	o.log.Info("📦 website_agent: EXTRACTING FILES",
		logger.Bool("is_multi_file", len(files) > 0),
		logger.Int("file_count", len(files)))
	var htmlContent string
	var artifactID string

	if len(files) > 0 {
		o.log.Info("📁 website_agent: multi-file project detected",
			logger.Int("file_count", len(files)),
			logger.String("files", strings.Join(getFilePaths(files), ", ")))

		for attempt := 1; attempt <= maxWebsiteRepairAttempts; attempt++ {
			issues := validateWebsiteProjectFiles(files)
			if len(issues) == 0 {
				o.log.Info("✅ website_agent: all files valid, no repair needed")
				break
			}

			o.log.Warn("🔧 website_agent: REPAIR needed",
				logger.Int("attempt", attempt),
				logger.Int("issue_count", len(issues)),
				logger.String("issues", truncate(formatWebsiteFileIssues(issues), 500)))
			select {
			case outCh <- llm.Chunk{Event: "progress", Content: "Reparando archivos incompletos..."}:
			case <-ctx.Done():
				return
			}
			files = o.repairWebsiteProjectFiles(ctx, agent, files, issues, maxTokens)
			o.log.Info("🔧 website_agent: repair attempt done",
				logger.Int("attempt", attempt),
				logger.Int("file_count_after_repair", len(files)))
		}

		if remaining := validateWebsiteProjectFiles(files); len(remaining) > 0 {
			o.log.Error("❌ website_agent: REPAIR FAILED — files still invalid after all attempts",
				logger.Int("remaining_issues", len(remaining)),
				logger.String("issues", truncate(formatWebsiteFileIssues(remaining), 500)))

			message := "No pude completar un proyecto web válido. Detecté archivos incompletos después de reparar; probá pedirlo con menos secciones o un alcance más chico."
			o.saveWebsiteAssistantMessage(ctx, conversationID, agent.Model, message, "")

			select {
			case outCh <- llm.Chunk{Event: "error", Content: "No pude completar un proyecto web válido. Detecté archivos incompletos; probá pedirlo más simple o con menos secciones."}:
			case <-ctx.Done():
				return
			}
			select {
			case outCh <- llm.Chunk{Done: true}:
			case <-ctx.Done():
			}
			return
		}

		// Find entry HTML file
		for _, f := range files {
			if f.Path == "index.html" {
				htmlContent = f.Content
				break
			}
		}

		artifactID = o.saveWebsiteArtifact(ctx, conversationID, htmlContent, files)

		o.log.Info("🎉 website_agent: PROJECT COMPLETE",
			logger.String("artifact_id", artifactID),
			logger.Int("file_count", len(files)),
			logger.String("files", strings.Join(getFilePaths(files), ", ")))
	} else {
		// Fallback to single HTML extraction
		htmlContent, matchType := extractWebsiteHTML(fullContent)
		if htmlContent != "" {
			o.log.Info("📄 website_agent: single HTML extracted",
				logger.String("match_type", matchType),
				logger.Int("html_length", len(htmlContent)),
				logger.String("html_preview", truncate(htmlContent, 80)))
		} else {
			o.log.Warn("⚠️ website_agent: no HTML pattern matched", logger.String("preview", truncate(fullContent, 200)))
		}

		artifactID = o.saveWebsiteArtifact(ctx, conversationID, htmlContent, nil)
		o.log.Info("🎉 website_agent: SINGLE HTML COMPLETE",
			logger.String("artifact_id", artifactID),
			logger.Int("html_length", len(htmlContent)))
	}

	// Generate clean summary for chat message
	summary := o.generateProjectSummary(files, htmlContent)
	o.saveWebsiteAssistantMessage(ctx, conversationID, agent.Model, summary, artifactID)

	// Generate title after successful artifact creation
	if userContent != "" {
		go o.generateAndSaveTitle(ctx, conversationID, conversationID, userContent)
	}

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

	o.log.Info("🌐 website_agent: calling LLM Stream",
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
			o.log.Info("📨 website_agent: FIRST chunk received",
				logger.Int("attempt", attempt),
				logger.Int("chunk_number", chunkCount),
				logger.String("preview", truncate(chunk.Content, 80)))
		}
		chunkCount++

		if chunk.Content == "" {
			continue
		}
		contentBuilder.WriteString(chunk.Content)
		select {
		case outCh <- llm.Chunk{Content: chunk.Content}:
		case <-ctx.Done():
			o.log.Warn("⚠️ website_agent: context cancelled during streaming",
			logger.Int("attempt", attempt),
			logger.Int("chunks_received", chunkCount))
			return chunkCount, firstChunkReceived, ctx.Err()
		}
	}

	o.log.Info("📡 website_agent: stream ended",
		logger.Int("attempt", attempt),
		logger.Int("total_chunks", chunkCount),
		logger.Int("content_length", contentBuilder.Len()))

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
		case outCh <- llm.Chunk{
			Event:        "artifact",
			ArtifactID:   artifactID,
			ArtifactType: "website",
		}:
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

func (o *Orchestrator) generateProjectSummary(files []model.ArtifactFile, htmlContent string) string {
	if len(files) > 0 {
		fileList := strings.Join(getFilePaths(files), ", ")
		return fmt.Sprintf("🎨 **Landing Page generada**\n\nSe creó un proyecto web con %d archivos:\n- %s\n\nHacé clic en **'Ver en Sandbox'** para previsualizar y editar el sitio.", len(files), fileList)
	}
	if htmlContent != "" {
		return "🎨 **Landing Page generada**\n\nSe creó una página HTML. Hacé clic en **'Ver en Sandbox'** para previsualizar."
	}
	return "🎨 Se intentó generar una landing page pero no se encontró contenido válido."
}
