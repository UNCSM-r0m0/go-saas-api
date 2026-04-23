package model

import (
	"time"

	"github.com/google/uuid"
)

// AgentRole defines the type of agent.
type AgentRole string

const (
	RoleSystem       AgentRole = "system"
	RoleCoder        AgentRole = "coder"
	RoleCopywriter   AgentRole = "copywriter"
	RoleCritic       AgentRole = "critic"
	RoleResearcher   AgentRole = "researcher"
	RoleAssistant    AgentRole = "assistant"
)

// Agent is a configured AI agent or sub-agent.
type Agent struct {
	ID           uuid.UUID      `json:"id"`
	TenantID     uuid.UUID      `json:"tenant_id"`
	UserID       *uuid.UUID     `json:"user_id,omitempty"`
	Name         string         `json:"name"`
	Role         AgentRole      `json:"role"`
	Model        string         `json:"model"`
	SystemPrompt string         `json:"system_prompt"`
	ToolsEnabled []string       `json:"tools_enabled"`
	Settings     map[string]any `json:"settings"`
	ParentAgentID *uuid.UUID    `json:"parent_agent_id,omitempty"`
	IsActive     bool           `json:"is_active"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}
