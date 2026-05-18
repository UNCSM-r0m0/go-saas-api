package usage

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/billing"
)

// Tracker abstracts billing.UsageRepository for the agent runtime.
type Tracker struct {
	repo billing.UsageRepository
}

// NewTracker creates a new usage tracker. repo may be nil (no-op).
func NewTracker(repo billing.UsageRepository) *Tracker {
	return &Tracker{repo: repo}
}

// Record holds usage data for a single LLM interaction.
type Record struct {
	UserID         uuid.UUID
	ConversationID uuid.UUID
	Model          string
	Provider       string
	TokensInput    int
	TokensOutput   int
	LatencyMs      int
	CostUSD        float64
}

// Record persists a usage log and updates the daily rollup.
// Returns nil if the repository is not configured (no-op mode).
func (t *Tracker) Record(ctx context.Context, r Record) error {
	if t.repo == nil {
		return nil
	}
	log := &billing.UsageLog{
		ID:             uuid.New(),
		UserID:         r.UserID,
		ConversationID: &r.ConversationID,
		Model:          r.Model,
		Provider:       r.Provider,
		TokensInput:    r.TokensInput,
		TokensOutput:   r.TokensOutput,
		LatencyMs:      r.LatencyMs,
		CostUSD:        r.CostUSD,
		CreatedAt:      time.Now().UTC(),
	}
	if err := t.repo.LogUsage(ctx, log); err != nil {
		return err
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	return t.repo.IncrementDailyUsage(ctx, r.UserID, today, r.TokensInput, r.TokensOutput, r.CostUSD)
}
