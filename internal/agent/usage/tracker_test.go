package usage

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/billing"
)

type mockUsageRepo struct {
	logs         []*billing.UsageLog
	dailyUpdates []dailyUpdate
}

type dailyUpdate struct {
	userID     uuid.UUID
	date       time.Time
	tokensIn   int
	tokensOut  int
	costUSD    float64
}

func (m *mockUsageRepo) LogUsage(ctx context.Context, log *billing.UsageLog) error {
	m.logs = append(m.logs, log)
	return nil
}

func (m *mockUsageRepo) GetDailyUsage(ctx context.Context, userID uuid.UUID, date time.Time) (*billing.DailyUsage, error) {
	return nil, nil
}

func (m *mockUsageRepo) IncrementDailyUsage(ctx context.Context, userID uuid.UUID, date time.Time, tokensIn, tokensOut int, costUSD float64) error {
	m.dailyUpdates = append(m.dailyUpdates, dailyUpdate{userID, date, tokensIn, tokensOut, costUSD})
	return nil
}

func (m *mockUsageRepo) GetUsageStats(ctx context.Context, userID uuid.UUID, from, to time.Time) (*billing.UsageStats, error) {
	return &billing.UsageStats{}, nil
}

func TestTracker_Record(t *testing.T) {
	repo := &mockUsageRepo{}
	tracker := NewTracker(repo)
	ctx := context.Background()
	userID := uuid.New()
	convID := uuid.New()

	err := tracker.Record(ctx, Record{
		UserID:         userID,
		ConversationID: convID,
		Model:          "gpt-4o",
		TokensInput:    100,
		TokensOutput:   50,
		LatencyMs:      1200,
		CostUSD:        0.001,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(repo.logs) != 1 {
		t.Fatalf("expected 1 usage log, got %d", len(repo.logs))
	}
	log := repo.logs[0]
	if log.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, log.UserID)
	}
	if log.Model != "gpt-4o" {
		t.Errorf("expected model gpt-4o, got %s", log.Model)
	}
	if log.TokensInput != 100 {
		t.Errorf("expected tokens_input 100, got %d", log.TokensInput)
	}
	if log.TokensOutput != 50 {
		t.Errorf("expected tokens_output 50, got %d", log.TokensOutput)
	}
	if log.LatencyMs != 1200 {
		t.Errorf("expected latency_ms 1200, got %d", log.LatencyMs)
	}
	if log.CostUSD != 0.001 {
		t.Errorf("expected cost_usd 0.001, got %f", log.CostUSD)
	}

	if len(repo.dailyUpdates) != 1 {
		t.Fatalf("expected 1 daily update, got %d", len(repo.dailyUpdates))
	}
	du := repo.dailyUpdates[0]
	if du.tokensIn != 100 {
		t.Errorf("expected daily tokens_in 100, got %d", du.tokensIn)
	}
	if du.tokensOut != 50 {
		t.Errorf("expected daily tokens_out 50, got %d", du.tokensOut)
	}
	if du.costUSD != 0.001 {
		t.Errorf("expected daily cost 0.001, got %f", du.costUSD)
	}
}

func TestTracker_NilRepo(t *testing.T) {
	tracker := NewTracker(nil)
	ctx := context.Background()

	err := tracker.Record(ctx, Record{
		UserID:       uuid.New(),
		Model:        "gpt-4o",
		TokensInput:  100,
		TokensOutput: 50,
	})
	if err != nil {
		t.Fatalf("expected nil error for nil repo, got %v", err)
	}
}
