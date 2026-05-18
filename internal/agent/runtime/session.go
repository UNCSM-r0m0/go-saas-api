package runtime

import (
	"context"
	"fmt"
	"time"

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
func (s *SessionManager) CreateConversation(ctx context.Context, userID uuid.UUID, title string, agentID *uuid.UUID) (*model.Conversation, error) {
	conv := &model.Conversation{
		ID:      uuid.New(),
		UserID:  userID,
		Title:   title,
		AgentID: agentID,
		Status:  model.ConversationActive,
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

// UpdateConversationTitle updates the title of a conversation.
func (s *SessionManager) UpdateConversationTitle(ctx context.Context, id uuid.UUID, title string) error {
	conv, err := s.convRepo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	conv.Title = title
	conv.UpdatedAt = time.Now()
	return s.convRepo.Update(ctx, conv)
}

// GetConversation retrieves a conversation by ID.
func (s *SessionManager) GetConversation(ctx context.Context, conversationID uuid.UUID) (*model.Conversation, error) {
	return s.convRepo.GetByID(ctx, conversationID)
}

// GetHistory retrieves messages for a conversation.
func (s *SessionManager) GetHistory(ctx context.Context, conversationID uuid.UUID, limit int) ([]model.Message, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.msgRepo.ListByConversation(ctx, conversationID, limit)
}

// UpdateConversationMetadata updates only the metadata field of a conversation.
func (s *SessionManager) UpdateConversationMetadata(ctx context.Context, id uuid.UUID, metadata map[string]any) error {
	conv, err := s.convRepo.GetByID(ctx, id)
	if err != nil {
		return fmt.Errorf("get conversation: %w", err)
	}
	conv.Metadata = metadata
	conv.UpdatedAt = time.Now()
	return s.convRepo.Update(ctx, conv)
}
