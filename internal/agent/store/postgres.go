package store

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

func setTenant(ctx context.Context, pool *pgxpool.Pool, tenantID uuid.UUID) error {
	_, err := pool.Exec(ctx, "SELECT set_config('app.current_tenant', $1, false)", tenantID.String())
	return err
}

// ---- ConversationStore ----

type ConversationStore struct {
	pool *pgxpool.Pool
}

func NewConversationStore(pool *pgxpool.Pool) *ConversationStore {
	return &ConversationStore{pool: pool}
}

func (s *ConversationStore) Create(ctx context.Context, conv *model.Conversation) error {
	if err := setTenant(ctx, s.pool, conv.TenantID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO conversations (id, tenant_id, user_id, title, agent_id, status, metadata, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		conv.ID, conv.TenantID, conv.UserID, conv.Title, conv.AgentID, conv.Status, conv.Metadata, conv.CreatedAt, conv.UpdatedAt)
	return err
}

func (s *ConversationStore) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Conversation, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	var conv model.Conversation
	var agentID *uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, title, agent_id, status, metadata, created_at, updated_at
		 FROM conversations WHERE id = $1`, id).Scan(
		&conv.ID, &conv.TenantID, &conv.UserID, &conv.Title, &agentID, &conv.Status, &conv.Metadata, &conv.CreatedAt, &conv.UpdatedAt)
	if err != nil {
		return nil, err
	}
	conv.AgentID = agentID
	return &conv, nil
}

func (s *ConversationStore) ListByUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]model.Conversation, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, user_id, title, agent_id, status, metadata, created_at, updated_at
		 FROM conversations WHERE user_id = $1 AND status != 'deleted' ORDER BY updated_at DESC LIMIT $2 OFFSET $3`,
		userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Conversation
	for rows.Next() {
		var conv model.Conversation
		var agentID *uuid.UUID
		if err := rows.Scan(&conv.ID, &conv.TenantID, &conv.UserID, &conv.Title, &agentID, &conv.Status, &conv.Metadata, &conv.CreatedAt, &conv.UpdatedAt); err != nil {
			return nil, err
		}
		conv.AgentID = agentID
		list = append(list, conv)
	}
	return list, rows.Err()
}

func (s *ConversationStore) Update(ctx context.Context, conv *model.Conversation) error {
	if err := setTenant(ctx, s.pool, conv.TenantID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE conversations SET title = $1, agent_id = $2, status = $3, metadata = $4, updated_at = $5 WHERE id = $6`,
		conv.Title, conv.AgentID, conv.Status, conv.Metadata, conv.UpdatedAt, conv.ID)
	return err
}

func (s *ConversationStore) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE conversations SET status = 'deleted' WHERE id = $1`, id)
	return err
}

var _ repository.ConversationRepo = (*ConversationStore)(nil)

// ---- MessageStore ----

type MessageStore struct {
	pool *pgxpool.Pool
}

func NewMessageStore(pool *pgxpool.Pool) *MessageStore {
	return &MessageStore{pool: pool}
}

func (s *MessageStore) Create(ctx context.Context, msg *model.Message) error {
	if err := setTenant(ctx, s.pool, msg.TenantID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO messages (id, tenant_id, conversation_id, role, content, tool_calls, model, tokens_input, tokens_output, latency_ms, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		msg.ID, msg.TenantID, msg.ConversationID, msg.Role, msg.Content, msg.ToolCalls, msg.Model, msg.TokensInput, msg.TokensOutput, msg.LatencyMs, msg.CreatedAt)
	return err
}

func (s *MessageStore) ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, conversation_id, role, content, tool_calls, model, tokens_input, tokens_output, latency_ms, created_at
		 FROM messages WHERE conversation_id = $1 ORDER BY created_at ASC LIMIT $2`,
		conversationID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Message
	for rows.Next() {
		var msg model.Message
		if err := rows.Scan(&msg.ID, &msg.TenantID, &msg.ConversationID, &msg.Role, &msg.Content, &msg.ToolCalls, &msg.Model, &msg.TokensInput, &msg.TokensOutput, &msg.LatencyMs, &msg.CreatedAt); err != nil {
			return nil, err
		}
		list = append(list, msg)
	}
	return list, rows.Err()
}

var _ repository.MessageRepo = (*MessageStore)(nil)

// ---- ArtifactStore ----

type ArtifactStore struct {
	pool *pgxpool.Pool
}

func NewArtifactStore(pool *pgxpool.Pool) *ArtifactStore {
	return &ArtifactStore{pool: pool}
}

func (s *ArtifactStore) Create(ctx context.Context, art *model.Artifact) error {
	if err := setTenant(ctx, s.pool, art.TenantID); err != nil {
		return err
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO artifacts (id, tenant_id, conversation_id, message_id, name, type, language, content, version, is_deleted, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		art.ID, art.TenantID, art.ConversationID, art.MessageID, art.Name, art.Type, art.Language, art.Content, art.Version, art.IsDeleted, art.CreatedAt, art.UpdatedAt)
	return err
}

func (s *ArtifactStore) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Artifact, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	var art model.Artifact
	var msgID *uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, conversation_id, message_id, name, type, language, content, version, is_deleted, created_at, updated_at
		 FROM artifacts WHERE id = $1`, id).Scan(
		&art.ID, &art.TenantID, &art.ConversationID, &msgID, &art.Name, &art.Type, &art.Language, &art.Content, &art.Version, &art.IsDeleted, &art.CreatedAt, &art.UpdatedAt)
	if err != nil {
		return nil, err
	}
	art.MessageID = msgID
	return &art, nil
}

func (s *ArtifactStore) ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID) ([]model.Artifact, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx,
		`SELECT id, tenant_id, conversation_id, message_id, name, type, language, content, version, is_deleted, created_at, updated_at
		 FROM artifacts WHERE conversation_id = $1 AND is_deleted = false ORDER BY created_at DESC`,
		conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var list []model.Artifact
	for rows.Next() {
		var art model.Artifact
		var msgID *uuid.UUID
		if err := rows.Scan(&art.ID, &art.TenantID, &art.ConversationID, &msgID, &art.Name, &art.Type, &art.Language, &art.Content, &art.Version, &art.IsDeleted, &art.CreatedAt, &art.UpdatedAt); err != nil {
			return nil, err
		}
		art.MessageID = msgID
		list = append(list, art)
	}
	return list, rows.Err()
}

var _ repository.ArtifactRepo = (*ArtifactStore)(nil)

// ---- AgentStore ----

type AgentStore struct {
	pool *pgxpool.Pool
}

func NewAgentStore(pool *pgxpool.Pool) *AgentStore {
	return &AgentStore{pool: pool}
}

func (s *AgentStore) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Agent, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	var agent model.Agent
	var userID, parentID *uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, name, role, model, system_prompt, tools_enabled, settings, parent_agent_id, is_active, created_at, updated_at
		 FROM agents WHERE id = $1`, id).Scan(
		&agent.ID, &agent.TenantID, &userID, &agent.Name, &agent.Role, &agent.Model, &agent.SystemPrompt, &agent.ToolsEnabled, &agent.Settings, &parentID, &agent.IsActive, &agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		return nil, err
	}
	agent.UserID = userID
	agent.ParentAgentID = parentID
	return &agent, nil
}

func (s *AgentStore) GetDefault(ctx context.Context, tenantID uuid.UUID) (*model.Agent, error) {
	if err := setTenant(ctx, s.pool, tenantID); err != nil {
		return nil, err
	}
	var agent model.Agent
	var userID, parentID *uuid.UUID
	err := s.pool.QueryRow(ctx,
		`SELECT id, tenant_id, user_id, name, role, model, system_prompt, tools_enabled, settings, parent_agent_id, is_active, created_at, updated_at
		 FROM agents WHERE tenant_id = $1 AND role = 'system' AND is_active = true LIMIT 1`, tenantID).Scan(
		&agent.ID, &agent.TenantID, &userID, &agent.Name, &agent.Role, &agent.Model, &agent.SystemPrompt, &agent.ToolsEnabled, &agent.Settings, &parentID, &agent.IsActive, &agent.CreatedAt, &agent.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("no default agent found: %w", err)
	}
	agent.UserID = userID
	agent.ParentAgentID = parentID
	return &agent, nil
}

var _ repository.AgentRepo = (*AgentStore)(nil)
