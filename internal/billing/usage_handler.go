package billing

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	natsio "github.com/nats-io/nats.go"
)

// UsageHandler holds HTTP handlers for usage endpoints.
type UsageHandler struct {
	usageRepo UsageRepository
	plans     PlanRepository
	natsConn  *natsio.Conn
}

// NewUsageHandler creates a new usage handler.
func NewUsageHandler(usageRepo UsageRepository, plans PlanRepository, nc *natsio.Conn) *UsageHandler {
	return &UsageHandler{usageRepo: usageRepo, plans: plans, natsConn: nc}
}

// RegisterRoutes registers usage routes.
func (h *UsageHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/stats", h.handleGetStats)
	r.GET("/limits", h.handleGetLimits)
	r.POST("/track", h.handleTrackUsage)
}

func (h *UsageHandler) handleGetStats(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing headers"})
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	from := time.Now().UTC().AddDate(0, 0, -30)
	to := time.Now().UTC()

	stats, err := h.usageRepo.GetUsageStats(c.Request.Context(), userID, from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"stats": stats})
}

func (h *UsageHandler) handleGetLimits(c *gin.Context) {
	plans, err := h.plans.ListPlans(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list plans"})
		return
	}

	limits := make(map[string]gin.H)
	for _, p := range plans {
		limits[p.Slug] = gin.H{
			"messages_per_month":     p.MessagesPerMonth,
			"max_tokens_per_request": p.MaxTokensPerRequest,
			"features":               p.Features,
		}
	}

	c.JSON(http.StatusOK, gin.H{"limits": limits})
}

func (h *UsageHandler) handleTrackUsage(c *gin.Context) {
	var req struct {
		UserID         uuid.UUID  `json:"user_id" binding:"required"`
		ConversationID *uuid.UUID `json:"conversation_id"`
		Model          string     `json:"model" binding:"required"`
		Provider       string     `json:"provider" binding:"required"`
		TokensInput    int        `json:"tokens_input"`
		TokensOutput   int        `json:"tokens_output"`
		LatencyMs      int        `json:"latency_ms"`
		CostUSD        float64    `json:"cost_usd"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log := &UsageLog{
		ID:             uuid.New(),
		UserID:         req.UserID,
		ConversationID: req.ConversationID,
		Model:          req.Model,
		Provider:       req.Provider,
		TokensInput:    req.TokensInput,
		TokensOutput:   req.TokensOutput,
		LatencyMs:      req.LatencyMs,
		CostUSD:        req.CostUSD,
		CreatedAt:      time.Now().UTC(),
	}

	if err := h.usageRepo.LogUsage(c.Request.Context(), log); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update daily rollup
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := h.usageRepo.IncrementDailyUsage(c.Request.Context(), req.UserID, today,
		req.TokensInput, req.TokensOutput, req.CostUSD); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Publish usage.recorded event if NATS is enabled
	if h.natsConn != nil {
		event := map[string]interface{}{
			"user_id":       req.UserID.String(),
			"tokens_input":  req.TokensInput,
			"tokens_output": req.TokensOutput,
			"cost_usd":      req.CostUSD,
			"timestamp":     time.Now().UTC().Format(time.RFC3339),
		}
		data, _ := json.Marshal(event)
		_ = h.natsConn.Publish("usage.recorded", data)
	}

	c.JSON(http.StatusCreated, gin.H{"status": "tracked"})
}
