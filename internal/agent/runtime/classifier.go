package runtime

import (
	"context"
	"fmt"
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// LLMClassifier uses an LLM to determine the best agent role for a user message.
type LLMClassifier struct {
	client llm.Client
}

// NewLLMClassifier creates a new LLM-based intent classifier.
func NewLLMClassifier(client llm.Client) *LLMClassifier {
	return &LLMClassifier{client: client}
}

// Classify determines the best agent role for a user message using an LLM.
func (c *LLMClassifier) Classify(ctx context.Context, message string) (model.AgentRole, error) {
	prompt := fmt.Sprintf(`Classify the user intent into ONE of these roles:
- coder: writing code, creating apps, HTML, CSS, JavaScript, Python, etc.
- researcher: finding information, investigating topics, searching
- copywriter: writing text, articles, marketing, descriptions
- assistant: general conversation, help, questions

User message: %q

Respond with ONLY the role name, nothing else.`, message)

	req := llm.Request{
		Model:       "qwen2.5-coder:7b",
		Messages:    []llm.Message{{Role: "user", Content: prompt}},
		Temperature: 0.0,
		MaxTokens:   20,
		Stream:      false,
	}

	resp, err := c.client.Complete(ctx, req)
	if err != nil {
		return model.RoleAssistant, err
	}

	roleStr := strings.TrimSpace(strings.ToLower(resp))
	switch roleStr {
	case "coder":
		return model.RoleCoder, nil
	case "researcher":
		return model.RoleResearcher, nil
	case "copywriter":
		return model.RoleCopywriter, nil
	default:
		return model.RoleAssistant, nil
	}
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
