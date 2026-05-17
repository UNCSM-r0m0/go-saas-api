package billing

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/r0lm0/go-saas-api/internal/platform/response"
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

	// r3-chat frontend compatible Stripe routes
	r.GET("/stripe/subscription", h.handleStripeSubscription)
	r.POST("/stripe/create-checkout-session", h.handleStripeCreateCheckout)
	r.POST("/stripe/create-portal-session", h.handleStripeCreatePortal)
	r.POST("/stripe/confirm-session", h.handleStripeConfirmSession)
	r.POST("/subscriptions", h.handleCreateSubscription)
	r.POST("/subscriptions/cancel", h.handleCancel)
}

func (h *Handler) handleListPlans(c *gin.Context) {
	plans, err := h.service.plans.ListPlans(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "failed to list plans")
		return
	}
	response.OK(c, plans, "plans retrieved")
}

func (h *Handler) handleSubscribe(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	var req struct {
		Email    string `json:"email" binding:"required,email"`
		PlanSlug string `json:"plan_slug" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	url, err := h.service.CreateCheckoutSession(c.Request.Context(), userID, req.Email, req.PlanSlug)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{"checkout_url": url}, "checkout session created")
}

func (h *Handler) handleGetSubscription(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	sub, plan, err := h.service.GetSubscription(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{
		"subscription": sub,
		"plan":         plan,
	}, "subscription retrieved")
}

func (h *Handler) handleCancel(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	if err := h.service.CancelSubscription(c.Request.Context(), userID); err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, nil, "subscription cancelled")
}

func (h *Handler) handleWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid payload")
		return
	}

	sig := c.GetHeader("Stripe-Signature")
	if sig == "" {
		response.Error(c, http.StatusBadRequest, "missing signature")
		return
	}

	if err := h.service.HandleWebhook(c.Request.Context(), WebhookPayload{
		Payload:   payload,
		Signature: sig,
	}); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	response.OK(c, nil, "webhook processed")
}

// --- r3-chat frontend compatible handlers ---

func (h *Handler) handleStripeSubscription(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	sub, plan, err := h.service.GetSubscription(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{
		"subscription": sub,
		"plan":         plan,
	}, "subscription retrieved")
}

func (h *Handler) handleStripeCreateCheckout(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	var req struct {
		PriceID string `json:"priceId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Map priceId to plan slug. For now, treat any priceId as "premium".
	url, err := h.service.CreateCheckoutSession(c.Request.Context(), userID, "", req.PriceID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{"url": url, "sessionId": ""}, "checkout session created")
}

func (h *Handler) handleStripeCreatePortal(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	// Get subscription to find stripe customer ID
	sub, _, err := h.service.GetSubscription(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	if sub == nil || sub.StripeCustomerID == "" {
		response.Error(c, http.StatusBadRequest, "no active subscription found")
		return
	}

	// Create portal session via Stripe API
	portalURL, err := h.service.CreatePortalSession(c.Request.Context(), sub.StripeCustomerID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{"url": portalURL}, "portal session created")
}

func (h *Handler) handleStripeConfirmSession(c *gin.Context) {
	var req struct {
		SessionID string `json:"sessionId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// For now, just acknowledge. In a full implementation, this would verify
	// the Stripe checkout session status.
	response.OK(c, gin.H{"status": "confirmed", "sessionId": req.SessionID}, "session confirmed")
}

func (h *Handler) handleCreateSubscription(c *gin.Context) {
	userIDStr := c.GetHeader("X-User-ID")
	if userIDStr == "" {
		response.Error(c, http.StatusUnauthorized, "missing headers")
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "invalid user_id")
		return
	}

	var req struct {
		Plan string `json:"plan" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	// Create a checkout session for the requested plan
	url, err := h.service.CreateCheckoutSession(c.Request.Context(), userID, "", req.Plan)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}

	response.OK(c, gin.H{"checkout_url": url}, "subscription checkout created")
}
