package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// LLMClassifier uses an LLM to determine the best agent role and flow for a user message.
type LLMClassifier struct {
	client llm.Client
}

// NewLLMClassifier creates a new LLM-based intent classifier.
func NewLLMClassifier(client llm.Client) *LLMClassifier {
	return &LLMClassifier{client: client}
}

// Classify determines the best agent role and optional flow for a user message using an LLM.
func (c *LLMClassifier) Classify(ctx context.Context, message string) (model.ClassificationResult, error) {
	prompt := fmt.Sprintf(`Analyze the user message and classify it.

ROLES (pick one):
- coder: writing code, creating apps, HTML, CSS, JavaScript, Python, etc.
- researcher: finding information, investigating topics, searching
- copywriter: writing text, articles, marketing, descriptions
- architect: system design, architecture decisions, tech choices
- assistant: general conversation, help, questions

FLOWS (pick one if the message maps to a structured workflow, otherwise null):
- onboarding: first-time setup, getting started
- proposal: proposal, quote, budget, presupuesto, cotización
- contract: contract, agreement, contrato, acuerdo, NDA
- code_review: code review, audit, revisar código
- api_integration: API integration, WooCommerce, Shopify, ERP sync, webhook, connect systems

Output ONLY a JSON object, no explanation:
{"role": "coder", "flow": null}

User message: %q`, message)

	req := llm.Request{
		Model:       "qwen2.5-coder:7b",
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.0,
		MaxTokens:   60,
		Stream:      false,
	}

	resp, err := c.client.Complete(ctx, req)
	if err != nil {
		return model.ClassificationResult{Role: model.RoleAssistant}, err
	}

	return parseClassification(resp), nil
}

func parseClassification(raw string) model.ClassificationResult {
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return model.ClassificationResult{Role: model.RoleAssistant}
	}

	var result struct {
		Role string `json:"role"`
		Flow string `json:"flow"`
	}
	if err := json.Unmarshal([]byte(raw[start:end+1]), &result); err != nil {
		return model.ClassificationResult{Role: model.RoleAssistant}
	}

	role := model.RoleAssistant
	switch strings.TrimSpace(strings.ToLower(result.Role)) {
	case "coder":
		role = model.RoleCoder
	case "researcher":
		role = model.RoleResearcher
	case "copywriter":
		role = model.RoleCopywriter
	case "architect":
		role = model.RoleArchitect
	}

	flow := ""
	if result.Flow != "" && result.Flow != "null" {
		flow = strings.TrimSpace(strings.ToLower(result.Flow))
	}

	return model.ClassificationResult{Role: role, Flow: flow}
}

// ClassifyKeyword determines the best agent role using simple keyword matching.
// This is kept as a lightweight fallback when the LLM is unavailable.
func ClassifyKeyword(message string) model.AgentRole {
	lower := strings.ToLower(message)

	coderKeywords := []string{
		"create html", "write html", "build html", "generate html",
		"create css", "write css", "build css",
		"create js", "write js", "build javascript",
		"write code", "build code", "generate code",
		"create app", "build app", "write script",
		"programa", "código", "html", "css", "javascript", "python", "script",
	}
	for _, kw := range coderKeywords {
		if strings.Contains(lower, kw) {
			return model.RoleCoder
		}
	}

	researcherKeywords := []string{
		"research", "investiga", "busca", "find information",
		"look up", "search for",
	}
	for _, kw := range researcherKeywords {
		if strings.Contains(lower, kw) {
			return model.RoleResearcher
		}
	}

	copywriterKeywords := []string{
		"write text", "copy", "blog post", "article",
		"email", "marketing", "description",
	}
	for _, kw := range copywriterKeywords {
		if strings.Contains(lower, kw) {
			return model.RoleCopywriter
		}
	}

	return model.RoleAssistant
}

// Classify is an alias for ClassifyKeyword for backward compatibility.
// Deprecated: use LLMClassifier.Classify or ClassifyKeyword explicitly.
func Classify(message string) model.AgentRole {
	return ClassifyKeyword(message)
}
