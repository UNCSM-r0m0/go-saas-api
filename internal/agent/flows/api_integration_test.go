package flows

import (
	"strings"
	"testing"

	"github.com/r0lm0/go-saas-api/internal/agent/model"
)

func TestAPIIntegrationFlow_Detect(t *testing.T) {
	f := NewAPIIntegrationFlow()

	tests := []struct {
		msg       string
		wantMatch bool
		minScore  float64
	}{
		{"I need a WooCommerce integration", true, 0.90},
		{"quiero integrar mi tienda Shopify con el ERP", true, 0.90},
		{"necesito sincronizar pedidos via webhook", true, 0.85},
		{"conectar WordPress con nuestro sistema", true, 0.90},
		{"API integration for inventory sync", true, 0.90},
		{"hola, cÃ³mo estÃ¡s?", false, 0},
		{"just a regular chat message", false, 0},
	}

	for _, tt := range tests {
		matched, score := f.Detect(tt.msg, 0)
		if matched != tt.wantMatch {
			t.Errorf("Detect(%q): matched=%v, want=%v", tt.msg, matched, tt.wantMatch)
		}
		if matched && score < tt.minScore {
			t.Errorf("Detect(%q): score=%f, want >= %f", tt.msg, score, tt.minScore)
		}
	}
}

func TestAPIIntegrationFlow_UpdateState_ExtractsFields(t *testing.T) {
	f := NewAPIIntegrationFlow()

	content := "I want to integrate WooCommerce REST API v3 to sync orders.\n\n---FLOW_STATE---\n{" +
		"\"extracted_fields\":{\"target_platform\":\"WooCommerce\",\"external_api_name\":\"WC REST API v3\",\"data_entities\":\"orders\"}," +
		"\"step\":\"collecting\"}"

	meta := f.UpdateState(nil, content)
	state := f.loadState(meta)

	if state.CollectedFields["target_platform"] != "WooCommerce" {
		t.Errorf("expected target_platform=WooCommerce, got %q", state.CollectedFields["target_platform"])
	}
	if state.CollectedFields["external_api_name"] != "WC REST API v3" {
		t.Errorf("expected external_api_name='WC REST API v3', got %q", state.CollectedFields["external_api_name"])
	}
	if state.CollectedFields["data_entities"] != "orders" {
		t.Errorf("expected data_entities='orders', got %q", state.CollectedFields["data_entities"])
	}
}

func TestAPIIntegrationFlow_MissingFieldsPriority(t *testing.T) {
	f := NewAPIIntegrationFlow()

	meta := map[string]any{
		"flow": "api_integration",
		"collected_fields": map[string]string{
			"target_platform":   "Shopify",
			"external_api_name": "Shopify Admin API",
		},
	}

	state := f.loadState(meta)
	missing := f.computeMissingFields(state.CollectedFields)

	if len(missing) == 0 {
		t.Fatal("expected missing fields, got none")
	}
	if missing[0] != "integration_goal" {
		t.Fatalf("expected integration_goal first, got %s", missing[0])
	}
}

func TestAPIIntegrationFlow_StatusTransitions(t *testing.T) {
	f := NewAPIIntegrationFlow()

	// 1. collecting -> ready_to_generate when all required fields present
	meta := map[string]any{
		"flow":    "api_integration",
		"status":  "collecting",
		"step":    "collecting",
		"collected_fields": map[string]string{
			"target_platform":   "WooCommerce",
			"external_api_name": "WC REST API v3",
			"integration_goal":  "Sync orders",
			"data_direction":    "import",
			"auth_type":         "api_key",
			"data_entities":     "orders",
			"frequency":         "real_time",
			"deliverable_type":  "script",
		},
	}

	updated := f.UpdateState(meta, "---FLOW_STATE---\n{\"step\":\"collecting\"}")
	state := f.loadState(updated)

	if state.Status != "ready_to_generate" {
		t.Fatalf("expected status ready_to_generate, got %s", state.Status)
	}
	if state.Step != "generating" {
		t.Fatalf("expected step generating, got %s", state.Step)
	}

	// 2. ready_to_generate -> generated when real deliverables are added
	updated2 := f.AddRealDeliverables(updated, []string{"01-project-scope.md"})
	state2 := f.loadState(updated2)

	if state2.Status != "generated" {
		t.Fatalf("expected status generated, got %s", state2.Status)
	}
	if len(state2.Deliverables) != 1 {
		t.Fatalf("expected 1 deliverable, got %d", len(state2.Deliverables))
	}

	// 3. collecting stays collecting when fields missing
	meta3 := map[string]any{
		"flow":   "api_integration",
		"status": "collecting",
		"collected_fields": map[string]string{
			"target_platform": "WooCommerce",
		},
	}
	updated3 := f.UpdateState(meta3, "")
	state3 := f.loadState(updated3)
	if state3.Status != "collecting" {
		t.Fatalf("expected status collecting, got %s", state3.Status)
	}
}

func TestAPIIntegrationFlow_BuildMessages_IncludesState(t *testing.T) {
	f := NewAPIIntegrationFlow()
	agent := &model.Agent{Role: model.RoleAssistant}

	meta := map[string]any{
		"collected_fields": map[string]string{"target_platform": "Shopify"},
		"missing_fields":   []string{"external_api_name", "integration_goal"},
		"status":           "collecting",
	}

	msgs := f.BuildMessages(agent, nil, "hello", "", meta)
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "system" {
		t.Fatal("expected system message first")
	}
	if !strings.Contains(msgs[0].Content, "Shopify") {
		t.Error("expected system prompt to mention collected platform")
	}
	if !strings.Contains(msgs[0].Content, "external_api_name") {
		t.Error("expected system prompt to mention next missing field")
	}
}

func TestAPIIntegrationFlow_ParseFlowStateBlock(t *testing.T) {
	f := NewAPIIntegrationFlow()

	content := "Some assistant response here.\n\n---FLOW_STATE---\n{\"extracted_fields\":{\"auth_type\":\"oauth\"},\"step\":\"collecting\"}"
	block := f.parseFlowStateBlock(content)

	if block == nil {
		t.Fatal("expected parsed block, got nil")
	}
	if block.ExtractedFields["auth_type"] != "oauth" {
		t.Errorf("expected auth_type='oauth', got %q", block.ExtractedFields["auth_type"])
	}
}

func TestAPIIntegrationFlow_ParseFlowStateBlock_Missing(t *testing.T) {
	f := NewAPIIntegrationFlow()

	content := "Some assistant response without state block."
	block := f.parseFlowStateBlock(content)

	if block != nil {
		t.Error("expected nil block when no ---FLOW_STATE--- present")
	}
}

func TestAPIIntegrationFlow_Name(t *testing.T) {
	f := NewAPIIntegrationFlow()
	if f.Name() != "api_integration" {
		t.Fatalf("expected api_integration, got %s", f.Name())
	}
}

func TestAPIIntegrationFlow_InferFieldsFromContent(t *testing.T) {
	f := NewAPIIntegrationFlow()
	state := &apiIntegrationState{
		CollectedFields: make(map[string]string),
	}

	content := "We will use OAuth for authentication and import data in real time."
	f.inferFieldsFromContent(state, content)

	if state.CollectedFields["auth_type"] != "oauth" {
		t.Errorf("expected auth_type='oauth', got %q", state.CollectedFields["auth_type"])
	}
	if state.CollectedFields["data_direction"] != "import" {
		t.Errorf("expected data_direction='import', got %q", state.CollectedFields["data_direction"])
	}
	if state.CollectedFields["frequency"] != "real_time" {
		t.Errorf("expected frequency='real_time', got %q", state.CollectedFields["frequency"])
	}
}

func TestStripFlowStateBlock(t *testing.T) {
	raw := "Hello!\n\n---FLOW_STATE---\n{\"step\":\"collecting\"}"
	visible := StripFlowStateBlock(raw)
	if strings.Contains(visible, "---FLOW_STATE---") {
		t.Error("visible content should not contain FLOW_STATE block")
	}
	if visible != "Hello!" {
		t.Errorf("expected 'Hello!', got %q", visible)
	}
}

func TestStripFlowStateBlock_NoBlock(t *testing.T) {
	content := "Just a normal message."
	visible := StripFlowStateBlock(content)
	if visible != content {
		t.Errorf("expected unchanged content, got %q", visible)
	}
}

func TestEnforceSingleQuestion(t *testing.T) {
	tests := []struct {
		input    string
		status   string
		expected string
	}{
		{"What platform? It's WooCommerce.", "collecting", "What platform? It's WooCommerce."},
		{"What platform?", "collecting", "What platform?"},
		{"No questions here.", "collecting", "No questions here."},
		{"What? Why? How?", "ready_to_generate", "What? Why? How?"}, // no enforce outside collecting
		{"What is the API? Also, what is the goal?", "collecting", "What is the API?"},
	}

	for _, tt := range tests {
		got := EnforceSingleQuestion(tt.input, tt.status)
		if got != tt.expected {
			t.Errorf("EnforceSingleQuestion(%q, %q) = %q, want %q", tt.input, tt.status, got, tt.expected)
		}
	}
}

func TestAPIIntegrationFlow_MetadataContract(t *testing.T) {
	f := NewAPIIntegrationFlow()
	meta := f.UpdateState(nil, "---FLOW_STATE---\n{\"extracted_fields\":{\"target_platform\":\"Shopify\"}}")

	requiredKeys := []string{"flow", "step", "status", "collected_fields", "missing_fields", "suggested_options", "deliverables"}
	for _, key := range requiredKeys {
		if _, ok := meta[key]; !ok {
			t.Errorf("metadata missing required key: %s", key)
		}
	}

	if meta["flow"] != "api_integration" {
		t.Errorf("expected flow=api_integration, got %v", meta["flow"])
	}
	if meta["status"] != "collecting" {
		t.Errorf("expected status=collecting, got %v", meta["status"])
	}
}

func TestAPIIntegrationFlow_MergeUniqueDeliverables(t *testing.T) {
	base := []string{"01-project-scope.md"}
	incoming := []string{"01-project-scope.md", "02-technical-plan.md"}
	result := mergeUnique(base, incoming)

	if len(result) != 2 {
		t.Fatalf("expected 2 deliverables, got %d", len(result))
	}
	if result[0] != "01-project-scope.md" || result[1] != "02-technical-plan.md" {
		t.Errorf("unexpected deliverables: %v", result)
	}
}

func TestAPIIntegrationFlow_MetadataPreservation(t *testing.T) {
	f := NewAPIIntegrationFlow()
	meta := map[string]any{
		"model": "kimi",
		"flow":  "api_integration",
	}
	updated := f.UpdateState(meta, "---FLOW_STATE---\n{\"extracted_fields\":{\"target_platform\":\"Shopify\"}}")
	if updated["model"] != "kimi" {
		t.Errorf("expected model=kimi preserved, got %v", updated["model"])
	}
}

func TestAPIIntegrationFlow_FLOW_STATE_Deliverables_Ignored(t *testing.T) {
	f := NewAPIIntegrationFlow()
	meta := map[string]any{
		"flow":             "api_integration",
		"status":           "ready_to_generate",
		"collected_fields": map[string]string{"target_platform": "WooCommerce"},
	}
	// FLOW_STATE claims deliverables but no real file_write happened
	updated := f.UpdateState(meta, "---FLOW_STATE---\n{\"deliverables\":[\"01-project-scope.md\"],\"step\":\"generating\"}")
	state := f.loadState(updated)
	if state.Status == "generated" {
		t.Error("status should NOT be generated from FLOW_STATE deliverables alone")
	}
	if len(state.Deliverables) != 0 {
		t.Errorf("expected 0 deliverables, got %d", len(state.Deliverables))
	}
}

