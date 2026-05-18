package billing

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// compile-time check: PostgresBillingStore implements UsageRepository
var _ UsageRepository = (*PostgresBillingStore)(nil)

func TestUsageLog_HasCostFields(t *testing.T) {
	log := &UsageLog{
		ID:           uuid.New(),
		UserID:       uuid.New(),
		Model:        "gpt-4o",
		Provider:     "openai",
		TokensInput:  100,
		TokensOutput: 50,
		LatencyMs:    1200,
		CostUSD:      0.00125,
		CreatedAt:    time.Now().UTC(),
	}

	assert.Equal(t, "openai", log.Provider)
	assert.Equal(t, 1200, log.LatencyMs)
	assert.InDelta(t, 0.00125, log.CostUSD, 0.00001)
}

func TestDailyUsage_HasCostUSD(t *testing.T) {
	du := &DailyUsage{
		UserID:       uuid.New(),
		Date:         time.Now().UTC(),
		Requests:     5,
		TokensInput:  1000,
		TokensOutput: 500,
		CostUSD:      0.0125,
	}
	assert.InDelta(t, 0.0125, du.CostUSD, 0.00001)
}

func TestUsageStats_HasTotalCostUSD(t *testing.T) {
	stats := &UsageStats{
		TotalRequests: 10,
		TokensInput:   2000,
		TokensOutput:  1000,
		TotalCostUSD:  0.025,
	}
	assert.InDelta(t, 0.025, stats.TotalCostUSD, 0.00001)
}
