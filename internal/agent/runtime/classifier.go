package runtime

import (
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
)

// Classify determines the best agent role for a user message.
func Classify(message string) model.AgentRole {
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
