package flows

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/pkg/llm"
)

// APIIntegrationFlow guides users through scoping and delivering API integration projects.
type APIIntegrationFlow struct{}

// NewAPIIntegrationFlow creates a new API integration flow.
func NewAPIIntegrationFlow() *APIIntegrationFlow {
	return &APIIntegrationFlow{}
}

// Name returns the flow identifier.
func (f *APIIntegrationFlow) Name() string { return "api_integration" }

var requiredFields = []string{
	"target_platform",
	"external_api_name",
	"integration_goal",
	"data_direction",
	"auth_type",
	"data_entities",
	"frequency",
	"deliverable_type",
}

var optionalFields = []string{
	"budget_range",
	"deadline",
	"client_country",
	"api_docs_url",
	"existing_stack",
	"deployment_environment",
}

var suggestedOptions = map[string][]string{
	"target_platform":  {"WooCommerce", "Shopify", "WordPress", "Custom"},
	"data_direction":   {"import", "export", "bidirectional"},
	"auth_type":        {"api_key", "oauth", "basic_auth", "unknown"},
	"data_entities":    {"products", "orders", "customers", "inventory", "custom"},
	"frequency":        {"real_time", "scheduled", "manual"},
	"deliverable_type": {"plugin", "script", "backend_service", "documentation_only"},
}

// apiIntegrationState is the structured state persisted in conversation metadata.
type apiIntegrationState struct {
	Flow             string              `json:"flow"`
	Step             string              `json:"step"`              // internal: "collecting" | "generating"
	Status           string              `json:"status"`            // frontend contract: "collecting" | "ready_to_generate" | "generated"
	CollectedFields  map[string]string   `json:"collected_fields"`
	MissingFields    []string            `json:"missing_fields"`
	SuggestedOptions map[string][]string `json:"suggested_options"`
	Deliverables     []string            `json:"deliverables"`
}

// flowStateBlock is the JSON block appended by the assistant for state extraction.
type flowStateBlock struct {
	ExtractedFields map[string]string `json:"extracted_fields"`
	MissingFields   []string          `json:"missing_fields,omitempty"`
	Step            string            `json:"step,omitempty"`
	Deliverables    []string          `json:"deliverables,omitempty"`
}

// StripFlowStateBlock removes the ---FLOW_STATE--- block from assistant content.
func StripFlowStateBlock(content string) string {
	idx := strings.Index(content, "---FLOW_STATE---")
	if idx == -1 {
		return content
	}
	return strings.TrimSpace(content[:idx])
}

// EnforceSingleQuestion truncates assistant content to at most one question
// when the flow is still collecting fields. It is conservative: only truncates
// if it can find a clear sentence boundary after the first question.
func EnforceSingleQuestion(content string, status string) string {
	if status != "collecting" {
		return content
	}
	if strings.Count(content, "?") <= 1 {
		return content
	}
	firstQ := strings.Index(content, "?")
	if firstQ == -1 {
		return content
	}
	secondQ := strings.Index(content[firstQ+1:], "?")
	if secondQ == -1 {
		return content
	}
	secondQ += firstQ + 1

	// Look for a sentence boundary between first and second question
	cutoff := secondQ
	for i := secondQ - 1; i > firstQ; i-- {
		if content[i] == '\n' || content[i] == '.' || content[i] == '!' {
			cutoff = i + 1
			break
		}
	}
	// If no boundary found, cut right after the first question mark
	if cutoff == secondQ {
		cutoff = firstQ + 1
	}
	return strings.TrimSpace(content[:cutoff])
}

// Detect identifies API integration intents.
func (f *APIIntegrationFlow) Detect(message string, historyLen int) (bool, float64) {
	lower := strings.ToLower(message)

	highConf := []string{
		"api integration", "integrar api", "woocommerce", "shopify",
		"erp sync", "sincronizar", "conectar", "webhook", "rest api",
		"graphql", "salesforce", "sap", "zapier", "n8n", "magento",
		"wordpress", "custom", "integración api",
	}
	for _, kw := range highConf {
		if strings.Contains(lower, kw) {
			return true, 0.90
		}
	}

	medConf := []string{"integración", "conector", "sync", "api", "conexión", "conectar sistemas"}
	for _, kw := range medConf {
		if strings.Contains(lower, kw) {
			return true, 0.85
		}
	}

	return false, 0
}

// SystemPrompt returns the API integration flow system prompt.
func (f *APIIntegrationFlow) SystemPrompt() string {
	return `You are a technical consultant helping a freelancer scope and deliver an API integration project.

WORKFLOW RULES (follow STRICTLY):
1. Ask EXACTLY ONE question per response. Never ask multiple questions at once.
2. If the user's message already contains information that answers a required field, extract it and do NOT ask about it again.
3. Always prioritize the FIRST missing required field from the list below.
4. Be concise and professional. Use the user's language (Spanish or English).
5. When suggesting options, mention 2-3 most relevant ones.
6. When ALL required fields are collected, generate 5 deliverable documents using the file_write tool:
   - 01-project-scope.md (Project Scope & Objectives)
   - 02-technical-plan.md (Technical Implementation Plan)
   - 03-client-questions.md (Client Questions Checklist)
   - 04-delivery-readme.md (Delivery README)
   - 05-commercial-proposal.md (Commercial Proposal Draft)

REQUIRED FIELDS (must all be collected before generating):
1. target_platform - What platform/system needs to be integrated? (WooCommerce, Shopify, WordPress, Custom)
2. external_api_name - What is the specific external API name and version?
3. integration_goal - What should this integration achieve?
4. data_direction - Direction of data flow? (import, export, bidirectional)
5. auth_type - Authentication method? (api_key, oauth, basic_auth, unknown)
6. data_entities - What data entities sync? (products, orders, customers, inventory, custom)
7. frequency - How often does sync run? (real_time, scheduled, manual)
8. deliverable_type - What should be delivered? (plugin, script, backend_service, documentation_only)

OPTIONAL FIELDS (collect if the user mentions them, but don't force):
- budget_range, deadline, client_country, api_docs_url, existing_stack, deployment_environment

EXTRACTION FORMAT:
At the VERY END of EVERY response, you MUST append a ---FLOW_STATE--- block with JSON:

---FLOW_STATE---
{"extracted_fields":{"target_platform":"WooCommerce"},"missing_fields":["external_api_name","integration_goal"],"step":"collecting"}

You may also include "deliverables" when you generate documents:
{"extracted_fields":{},"deliverables":["01-project-scope.md"],"step":"generating"}

This block is invisible to the user but critical for tracking progress. NEVER omit it.`
}

// BuildMessages constructs messages with flow state tracking.
func (f *APIIntegrationFlow) BuildMessages(agent *model.Agent, history []model.Message, userMessage string, userContext string, metadata map[string]any) []llm.Message {
	state := f.loadState(metadata)

	systemContent := f.SystemPrompt()

	// Inject current state
	systemContent += "\n\n[CURRENT STATE]\n"
	systemContent += fmt.Sprintf("Collected fields: %v\n", state.CollectedFields)
	systemContent += fmt.Sprintf("Missing required fields: %v\n", state.MissingFields)
	systemContent += fmt.Sprintf("Status: %s\n", state.Status)

	if len(state.MissingFields) > 0 {
		nextField := state.MissingFields[0]
		opts, hasOpts := suggestedOptions[nextField]
		if hasOpts {
			systemContent += fmt.Sprintf("\nNEXT QUESTION: Ask about '%s'. Suggested options: %v\n", nextField, opts)
		} else {
			systemContent += fmt.Sprintf("\nNEXT QUESTION: Ask about '%s'.\n", nextField)
		}
	} else {
		systemContent += "\nALL REQUIRED FIELDS COLLECTED. Generate the 5 deliverable documents using file_write.\n"
	}

	if agent.SystemPrompt != "" {
		systemContent = agent.SystemPrompt + "\n\n" + systemContent
	}
	if userContext != "" {
		systemContent += "\n" + userContext
	}

	messages := []llm.Message{{Role: "system", Content: systemContent}}

	for _, h := range history {
		switch h.Role {
		case model.MessageRoleUser, model.MessageRoleSystem:
			messages = append(messages, llm.Message{Role: string(h.Role), Content: h.Content})
		case model.MessageRoleAssistant:
			messages = append(messages, llm.Message{Role: "assistant", Content: h.Content})
		}
	}

	messages = append(messages, llm.Message{Role: "user", Content: userMessage})
	return messages
}

// UpdateState extracts the next step from the LLM response and updates metadata.
func (f *APIIntegrationFlow) UpdateState(metadata map[string]any, assistantContent string) map[string]any {
	state := f.loadState(metadata)

	// Parse ---FLOW_STATE--- JSON block
	extracted := f.parseFlowStateBlock(assistantContent)
	if extracted != nil {
		for k, v := range extracted.ExtractedFields {
			if v != "" {
				state.CollectedFields[k] = v
			}
		}
		if extracted.Step != "" {
			state.Step = extracted.Step
		}
		// NOTE: we do NOT trust deliverables from FLOW_STATE block.
		// Real deliverables are tracked via successful file_write tool calls.
	}

	// Also attempt simple keyword extraction from the visible content as fallback
	f.inferFieldsFromContent(state, assistantContent)

	// Recompute missing fields
	state.MissingFields = f.computeMissingFields(state.CollectedFields)

	// Status transitions
	if len(state.MissingFields) == 0 && state.Status == "collecting" {
		state.Status = "ready_to_generate"
		state.Step = "generating"
	}
	// NOTE: status "generated" is ONLY set by AddRealDeliverables when
	// file_write tool calls actually succeed. Do NOT set it here.

	return f.saveState(state, metadata)
}

// loadState converts metadata map to structured state.
func (f *APIIntegrationFlow) loadState(metadata map[string]any) *apiIntegrationState {
	if metadata == nil {
		return &apiIntegrationState{
			Flow:             f.Name(),
			Step:             "collecting",
			Status:           "collecting",
			CollectedFields:  make(map[string]string),
			MissingFields:    append([]string(nil), requiredFields...),
			SuggestedOptions: suggestedOptions,
			Deliverables:     []string{},
		}
	}

	state := &apiIntegrationState{
		Flow:             f.Name(),
		Step:             "collecting",
		Status:           "collecting",
		CollectedFields:  make(map[string]string),
		MissingFields:    append([]string(nil), requiredFields...),
		SuggestedOptions: suggestedOptions,
		Deliverables:     []string{},
	}

	if v, ok := metadata["flow"].(string); ok {
		state.Flow = v
	}
	if v, ok := metadata["step"].(string); ok {
		state.Step = v
	}
	if v, ok := metadata["status"].(string); ok {
		state.Status = v
	}
	if v, ok := metadata["collected_fields"].(map[string]any); ok {
		for k, val := range v {
			if s, ok := val.(string); ok {
				state.CollectedFields[k] = s
			}
		}
	}
	if v, ok := metadata["collected_fields"].(map[string]string); ok {
		state.CollectedFields = v
	}
	if v, ok := metadata["missing_fields"].([]any); ok {
		state.MissingFields = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				state.MissingFields = append(state.MissingFields, s)
			}
		}
	}
	if v, ok := metadata["missing_fields"].([]string); ok {
		state.MissingFields = v
	}
	if v, ok := metadata["deliverables"].([]any); ok {
		state.Deliverables = make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				state.Deliverables = append(state.Deliverables, s)
			}
		}
	}
	if v, ok := metadata["deliverables"].([]string); ok {
		state.Deliverables = v
	}

	return state
}

// saveState converts structured state back to metadata map,
// merging over the original metadata to preserve unrelated keys.
func (f *APIIntegrationFlow) saveState(state *apiIntegrationState, original map[string]any) map[string]any {
	result := make(map[string]any, len(original)+7)
	for k, v := range original {
		result[k] = v
	}
	result["flow"] = state.Flow
	result["step"] = state.Step
	result["status"] = state.Status
	result["collected_fields"] = state.CollectedFields
	result["missing_fields"] = state.MissingFields
	result["suggested_options"] = state.SuggestedOptions
	result["deliverables"] = state.Deliverables
	return result
}

// GetStatus extracts the status value from metadata.
func GetStatus(metadata map[string]any) string {
	if v, ok := metadata["status"].(string); ok {
		return v
	}
	return "collecting"
}

// AddRealDeliverables merges filenames from successful file_write tool calls
// into the flow state and transitions status to "generated" when deliverables exist.
func (f *APIIntegrationFlow) AddRealDeliverables(metadata map[string]any, filenames []string) map[string]any {
	state := f.loadState(metadata)
	for _, fn := range filenames {
		found := false
		for _, existing := range state.Deliverables {
			if existing == fn {
				found = true
				break
			}
		}
		if !found {
			state.Deliverables = append(state.Deliverables, fn)
		}
	}
	if len(state.Deliverables) > 0 {
		state.Status = "generated"
	}
	return f.saveState(state, metadata)
}

// parseFlowStateBlock extracts the JSON block after ---FLOW_STATE---.
func (f *APIIntegrationFlow) parseFlowStateBlock(content string) *flowStateBlock {
	idx := strings.LastIndex(content, "---FLOW_STATE---")
	if idx == -1 {
		return nil
	}
	jsonPart := strings.TrimSpace(content[idx+len("---FLOW_STATE---"):])
	if jsonPart == "" {
		return nil
	}

	var block flowStateBlock
	if err := json.Unmarshal([]byte(jsonPart), &block); err != nil {
		return nil
	}
	return &block
}

// computeMissingFields returns required fields not yet collected.
func (f *APIIntegrationFlow) computeMissingFields(collected map[string]string) []string {
	var missing []string
	for _, field := range requiredFields {
		if collected[field] == "" {
			missing = append(missing, field)
		}
	}
	return missing
}

// inferFieldsFromContent attempts to extract field values from the visible assistant content.
// This is a lightweight fallback when the LLM forgets the ---FLOW_STATE--- block.
func (f *APIIntegrationFlow) inferFieldsFromContent(state *apiIntegrationState, content string) {
	lower := strings.ToLower(content)

	// target_platform inference
	if state.CollectedFields["target_platform"] == "" {
		platforms := map[string]string{
			"woocommerce": "WooCommerce",
			"shopify":     "Shopify",
			"wordpress":   "WordPress",
			"custom":      "Custom",
		}
		for kw, val := range platforms {
			if strings.Contains(lower, kw) {
				state.CollectedFields["target_platform"] = val
				break
			}
		}
	}

	// auth_type inference
	if state.CollectedFields["auth_type"] == "" {
		auths := map[string]string{
			"api key": "api_key", "apikey": "api_key",
			"oauth": "oauth", "oauth2": "oauth",
			"basic auth": "basic_auth", "basic_auth": "basic_auth",
		}
		for kw, val := range auths {
			if strings.Contains(lower, kw) {
				state.CollectedFields["auth_type"] = val
				break
			}
		}
	}

	// data_direction inference
	if state.CollectedFields["data_direction"] == "" {
		dirs := map[string]string{
			"import": "import", "export": "export",
			"bidirectional": "bidirectional", "both ways": "bidirectional",
		}
		for kw, val := range dirs {
			if strings.Contains(lower, kw) {
				state.CollectedFields["data_direction"] = val
				break
			}
		}
	}

	// frequency inference
	if state.CollectedFields["frequency"] == "" {
		freqs := map[string]string{
			"real time": "real_time", "real-time": "real_time", "webhook": "real_time",
			"scheduled": "scheduled", "batch": "scheduled", "cron": "scheduled",
			"manual": "manual", "on demand": "manual",
		}
		for kw, val := range freqs {
			if strings.Contains(lower, kw) {
				state.CollectedFields["frequency"] = val
				break
			}
		}
	}
}

func mergeUnique(base, incoming []string) []string {
	seen := make(map[string]bool, len(base))
	for _, b := range base {
		seen[b] = true
	}
	for _, inc := range incoming {
		if !seen[inc] {
			seen[inc] = true
			base = append(base, inc)
		}
	}
	return base
}
