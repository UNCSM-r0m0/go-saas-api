package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
)

// ConversationRepo handles conversation persistence.
type ConversationRepo interface {
	Create(ctx context.Context, conv *model.Conversation) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Conversation, error)
	ListByUser(ctx context.Context, tenantID, userID uuid.UUID, limit, offset int) ([]model.Conversation, error)
	Update(ctx context.Context, conv *model.Conversation) error
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// MessageRepo handles message persistence.
type MessageRepo interface {
	Create(ctx context.Context, msg *model.Message) error
	ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID, limit int) ([]model.Message, error)
}

// ArtifactRepo handles artifact persistence.
type ArtifactRepo interface {
	Create(ctx context.Context, art *model.Artifact) error
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Artifact, error)
	GetByName(ctx context.Context, tenantID, conversationID uuid.UUID, name string) (*model.Artifact, error)
	ListByConversation(ctx context.Context, tenantID, conversationID uuid.UUID) ([]model.Artifact, error)
}

// AgentRepo handles agent definitions.
type AgentRepo interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Agent, error)
	GetByRole(ctx context.Context, tenantID uuid.UUID, role model.AgentRole) (*model.Agent, error)
	GetDefault(ctx context.Context, tenantID uuid.UUID) (*model.Agent, error)
}
