package billing

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handler holds HTTP handlers for billing endpoints.
type Handler struct {
	service *BillingService
}

// NewHandler creates a new billing handler.
func NewHandler(service *BillingService) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers billing routes on the given router.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/plans", h.handleListPlans)
	r.POST("/subscribe", h.handleSubscribe)
	r.GET("/subscription", h.handleGetSubscription)
	r.POST("/cancel", h.handleCancel)
	r.POST("/webhook", h.handleWebhook)
}

func (h *Handler) handleListPlans(c *gin.Context) {
	plans, err := h.service.plans.ListPlans(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list plans"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"plans": plans})
}

func (h *Handler) handleSubscribe(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing headers"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		PlanSlug string `json:"plan_slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	url, err := h.service.CreateCheckoutSession(c.Request.Context(), tenantID, userID, req.Email, req.PlanSlug)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"checkout_url": url})
}

func (h *Handler) handleGetSubscription(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing headers"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	sub, plan, err := h.service.GetSubscription(c.Request.Context(), tenantID, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"subscription": sub,
		"plan":         plan,
	})
}

func (h *Handler) handleCancel(c *gin.Context) {
	tenantIDStr := c.GetHeader("X-Tenant-ID")
	userIDStr := c.GetHeader("X-User-ID")
	if tenantIDStr == "" || userIDStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "missing headers"})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid user_id"})
		return
	}

	if err := h.service.CancelSubscription(c.Request.Context(), tenantID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "canceled"})
}

func (h *Handler) handleWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	sig := c.GetHeader("Stripe-Signature")
	if sig == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing signature"})
		return
	}

	if err := h.service.HandleWebhook(c.Request.Context(), WebhookPayload{
		Payload:   payload,
		Signature: sig,
	}); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
