package runtime

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/agent/repository"
)

// SessionManager handles conversation lifecycle.
type SessionManager struct {
	convRepo repository.ConversationRepo
	msgRepo  repository.MessageRepo
}

// NewSessionManager creates a new session manager.
func NewSessionManager(convRepo repository.ConversationRepo, msgRepo repository.MessageRepo) *SessionManager {
	return &SessionManager{convRepo: convRepo, msgRepo: msgRepo}
}

// CreateConversation starts a new conversation.
func (s *SessionManager) CreateConversation(ctx context.Context, tenantID, userID uuid.UUID, title string, agentID *uuid.UUID) (*model.Conversation, error) {
	conv := &model.Conversation{
		ID:       uuid.New(),
		TenantID: tenantID,
		UserID:   userID,
		Title:    title,
		AgentID:  agentID,
		Status:   model.ConversationActive,
	}
	if err := s.convRepo.Create(ctx, conv); err != nil {
		return nil, fmt.Errorf("create conversation: %w", err)
	}
	return conv, nil
}

// AddMessage persists a message.
func (s *SessionManager) AddMessage(ctx context.Context, msg *model.Message) error {
	if err := s.msgRepo.Create(ctx, msg); err != nil {
		return fmt.Errorf("create message: %w", err)
	}
	return nil
}

// GetHistory retrieves messages for a conversation.
func (s *SessionManager) GetHistory(ctx context.Context, tenantID, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.msgRepo.ListByConversation(ctx, tenantID, conversationID, limit)
}
