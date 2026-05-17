package flows

import (
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// Flow is a structured conversational workflow (e.g. onboarding, proposal).
type Flow interface {
	// Name returns the unique flow identifier.
	Name() string

	// Detect determines if a user message should trigger this flow.
	// Returns confidence 0-1.
	Detect(message string, historyLen int) (bool, float64)

	// SystemPrompt returns the system prompt for this flow.
	SystemPrompt() string

	// BuildMessages constructs the message list for the LLM, including flow state.
	BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, metadata map[string]any) []llm.Message

	// UpdateState extracts the next step from the LLM response and updates metadata.
	UpdateState(metadata map[string]any, assistantContent string) map[string]any
}

// Registry holds registered flows.
type Registry struct {
	flows map[string]Flow
}

// NewRegistry creates an empty flow registry.
func NewRegistry() *Registry {
	return &Registry{flows: make(map[string]Flow)}
}

// Register adds a flow to the registry.
func (r *Registry) Register(f Flow) {
	r.flows[f.Name()] = f
}

// Get retrieves a flow by name.
func (r *Registry) Get(name string) (Flow, bool) {
	f, ok := r.flows[name]
	return f, ok
}

// DetectAll checks all registered flows and returns the best match.
func (r *Registry) DetectAll(message string, historyLen int) (Flow, float64) {
	var best Flow
	var bestScore float64
	for _, f := range r.flows {
		matched, score := f.Detect(message, historyLen)
		if matched && score > bestScore {
			best = f
			bestScore = score
		}
	}
	return best, bestScore
}
