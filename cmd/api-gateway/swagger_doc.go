package main

import (
	"github.com/gin-gonic/gin"
	"github.com/r0lm0/go-saas-api/internal/agent/model"
	"github.com/r0lm0/go-saas-api/internal/apikey"
	"github.com/r0lm0/go-saas-api/internal/auth"
	"github.com/r0lm0/go-saas-api/internal/billing"
	"github.com/r0lm0/go-saas-api/internal/fileupload"
	"github.com/r0lm0/go-saas-api/internal/provider"
)

// initSwagger forces the import of domain packages so swag can resolve their types.
// These variables are intentionally unused at runtime.
func initSwagger() {
	var (
		_ auth.RegisterRequest
		_ auth.LoginRequest
		_ auth.RefreshRequest
		_ auth.ForgotPasswordRequest
		_ auth.ResetPasswordRequest
		_ auth.User
		_ auth.TokenPair
		_ billing.Plan
		_ billing.Subscription
		_ billing.UsageLog
		_ billing.DailyUsage
		_ provider.AIProvider
		_ provider.AIModel
		_ provider.ProviderType
		_ apikey.APIKey
		_ fileupload.Upload
		_ model.Conversation
		_ model.Message
		_ model.Artifact
		_ model.ConversationStatus
		_ model.MessageRole
	)
}

// ---------------------------------------------------------------------------
// API metadata
// ---------------------------------------------------------------------------

// @title Go SaaS API
// @version 1.0
// @description API Gateway para el sistema SaaS de agentes IA con múltiples proveedores de LLM, autenticación JWT, billing Stripe y sandbox de ejecución de código.
// @termsOfService http://example.com/terms/
// @contact.name API Support
// @contact.email support@example.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:3000
// @BasePath /api/v1
// @schemes http
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.
func initSwaggerInfo() {}

// ---------------------------------------------------------------------------
// Response wrappers (local types so swag can reference them)
// ---------------------------------------------------------------------------

type registerResp struct {
	User auth.User `json:"user"`
}

type loginResp struct {
	User  auth.User    `json:"user"`
	Token auth.TokenPair `json:"token"`
}

type refreshResp struct {
	User  auth.User    `json:"user"`
	Token auth.TokenPair `json:"token"`
}

type logoutResp struct {
	Message string `json:"message"`
}

type forgotPasswordResp struct {
	Message string `json:"message"`
}

type resetPasswordResp struct {
	Message string `json:"message"`
}

type meResp struct {
	User auth.User `json:"user"`
}

type modelsListResp struct {
	Models []modelItem `json:"models"`
}

type modelItem struct {
	ID       string `json:"id"`
	Provider string `json:"provider"`
}

type chatReq struct {
	ConversationID *string   `json:"conversation_id"`
	Message        string    `json:"message" binding:"required"`
	FileIDs        []string  `json:"file_ids"`
}

type conversationsResp struct {
	Conversations []model.Conversation `json:"conversations"`
}

type messagesResp struct {
	Messages []model.Message `json:"messages"`
}

type artifactReq struct {
	ConversationID string `json:"conversation_id" binding:"required"`
	Name           string `json:"name" binding:"required"`
	Type           string `json:"type" binding:"required"`
	Language       string `json:"language"`
	Content        string `json:"content" binding:"required"`
}

type providersResp struct {
	Providers []provider.AIProvider `json:"providers"`
}

type providerModelsResp struct {
	Models []provider.AIModel `json:"models"`
}

type plansResp struct {
	Plans []billing.Plan `json:"plans"`
}

type subscribeReq struct {
	Email    string `json:"email" binding:"required,email"`
	PlanSlug string `json:"plan_slug" binding:"required"`
}

type subscribeResp struct {
	CheckoutURL string `json:"checkout_url"`
}

type subscriptionResp struct {
	Subscription *billing.Subscription `json:"subscription"`
	Plan         *billing.Plan         `json:"plan"`
}

type cancelResp struct {
	Status string `json:"status"`
}

type webhookResp struct {
	Status string `json:"status"`
}

type usageStatsResp struct {
	Stats []billing.DailyUsage `json:"stats"`
}

type usageLimitsResp struct {
	Limits map[string]limitInfo `json:"limits"`
}

type limitInfo struct {
	MessagesPerDay      int            `json:"messages_per_day"`
	MaxTokensPerRequest int            `json:"max_tokens_per_request"`
	Features            map[string]any `json:"features"`
}

type trackUsageReq struct {
	UserID         string   `json:"user_id" binding:"required"`
	ConversationID *string  `json:"conversation_id"`
	Model          string   `json:"model" binding:"required"`
	Provider       string   `json:"provider" binding:"required"`
	TokensInput    int      `json:"tokens_input"`
	TokensOutput   int      `json:"tokens_output"`
	LatencyMs      int      `json:"latency_ms"`
	CostUSD        float64  `json:"cost_usd"`
}

type trackUsageResp struct {
	Status string `json:"status"`
}

type apiKeysResp struct {
	Keys []apikey.APIKey `json:"keys"`
}

type apiKeyCreateReq struct {
	Name string `json:"name" binding:"required"`
}

type apiKeyCreateResp struct {
	Key        string    `json:"key"`
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Scopes     []string  `json:"scopes"`
	CreatedAt  string    `json:"created_at"`
}

type filesResp struct {
	Files []fileupload.Upload `json:"files"`
}

type healthResp struct {
	Status  string `json:"status"`
	Healthy bool   `json:"healthy"`
}

// ---------------------------------------------------------------------------
// Auth (public)
// ---------------------------------------------------------------------------

// @Summary Register a new user
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RegisterRequest true "Registration info"
// @Success 201 {object} registerResp
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Router /api/v1/auth/register [post]
func _swaggerAuthRegister(c *gin.Context) {}

// @Summary Login with email and password
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.LoginRequest true "Login credentials"
// @Success 200 {object} loginResp
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/login [post]
func _swaggerAuthLogin(c *gin.Context) {}

// @Summary Refresh access token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RefreshRequest true "Refresh token"
// @Success 200 {object} refreshResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/refresh [post]
func _swaggerAuthRefresh(c *gin.Context) {}

// @Summary Logout and invalidate refresh token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.RefreshRequest true "Refresh token"
// @Success 200 {object} logoutResp
// @Router /api/v1/auth/logout [post]
func _swaggerAuthLogout(c *gin.Context) {}

// @Summary Request password reset email
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.ForgotPasswordRequest true "Email address"
// @Success 200 {object} forgotPasswordResp
// @Router /api/v1/auth/forgot-password [post]
func _swaggerAuthForgotPassword(c *gin.Context) {}

// @Summary Reset password with token
// @Tags auth
// @Accept json
// @Produce json
// @Param request body auth.ResetPasswordRequest true "Reset token and new password"
// @Success 200 {object} resetPasswordResp
// @Failure 400 {object} map[string]string
// @Router /api/v1/auth/reset-password [post]
func _swaggerAuthResetPassword(c *gin.Context) {}

// @Summary Initiate Google OAuth login
// @Tags auth
// @Produce json
// @Success 307 {string} string "Redirects to Google"
// @Router /api/v1/auth/google [get]
func _swaggerAuthGoogle(c *gin.Context) {}

// @Summary Google OAuth callback
// @Tags auth
// @Produce json
// @Param code query string true "Authorization code"
// @Param state query string true "OAuth state"
// @Success 200 {object} loginResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/google/callback [get]
func _swaggerAuthGoogleCallback(c *gin.Context) {}

// @Summary Initiate GitHub OAuth login
// @Tags auth
// @Produce json
// @Success 307 {string} string "Redirects to GitHub"
// @Router /api/v1/auth/github [get]
func _swaggerAuthGitHub(c *gin.Context) {}

// @Summary GitHub OAuth callback
// @Tags auth
// @Produce json
// @Param code query string true "Authorization code"
// @Param state query string true "OAuth state"
// @Success 200 {object} loginResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/github/callback [get]
func _swaggerAuthGitHubCallback(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Auth (protected)
// ---------------------------------------------------------------------------

// @Summary Get current authenticated user
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} meResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/me [get]
func _swaggerAuthMe(c *gin.Context) {}

// @Summary Get current user profile (r3-chat alias)
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} meResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/profile [get]
func _swaggerAuthProfile(c *gin.Context) {}

// @Summary Unified OAuth callback (r3-chat)
// @Tags auth
// @Accept json
// @Produce json
// @Param request body map[string]string true "OAuth code and state"
// @Success 200 {object} loginResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/auth/callback [post]
func _swaggerAuthCallback(c *gin.Context) {}

// @Summary Update user profile
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]string true "Profile updates"
// @Success 200 {object} meResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/users/profile [put]
func _swaggerUpdateUserProfile(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Agent (public)
// ---------------------------------------------------------------------------

// @Summary List available AI models
// @Tags agent
// @Produce json
// @Success 200 {object} modelsListResp
// @Router /api/v1/chat/models [get]
func _swaggerChatModels(c *gin.Context) {}

// @Summary List public models (r3-chat format)
// @Tags agent
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/models/public [get]
func _swaggerModelsPublic(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Agent (protected)
// ---------------------------------------------------------------------------

// @Summary Chat with AI agent (SSE streaming)
// @Tags agent
// @Accept json
// @Produce plain
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body chatReq true "Chat message"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/agent/chat [post]
func _swaggerAgentChat(c *gin.Context) {}

// @Summary Send chat message (sync, r3-chat)
// @Tags agent
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body chatReq true "Chat message"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/chat/message [post]
func _swaggerChatMessage(c *gin.Context) {}

// @Summary Send chat message with streaming (r3-chat SSE)
// @Tags agent
// @Accept json
// @Produce plain
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body chatReq true "Chat message"
// @Success 200 {string} string "SSE stream"
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /api/v1/chat/message/stream [post]
func _swaggerChatMessageStream(c *gin.Context) {}

// @Summary Create a new chat (r3-chat)
// @Tags agent
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body map[string]string true "Chat title and model"
// @Success 201 {object} map[string]interface{}
// @Router /api/v1/chat [post]
func _swaggerCreateChat(c *gin.Context) {}

// @Summary Get chat with messages (r3-chat)
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param id path string true "Chat ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /api/v1/chat/{id} [get]
func _swaggerGetChat(c *gin.Context) {}

// @Summary List chat sessions (r3-chat alias)
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/chat/sessions [get]
func _swaggerChatSessions(c *gin.Context) {}

// @Summary WebSocket chat upgrade
// @Tags agent
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 101 {string} string "WebSocket upgrade"
// @Router /api/v1/agent/ws [get]
func _swaggerAgentWS(c *gin.Context) {}

// @Summary Create a new artifact
// @Tags agent
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body artifactReq true "Artifact data"
// @Success 201 {object} model.Artifact
// @Failure 400 {object} map[string]string
// @Router /api/v1/artifacts [post]
func _swaggerCreateArtifact(c *gin.Context) {}

// @Summary Preview artifact content
// @Tags agent
// @Produce plain
// @Security BearerAuth
// @Param id path string true "Artifact ID"
// @Success 200 {string} string "Artifact content"
// @Failure 404 {object} map[string]string
// @Router /api/v1/artifacts/{id}/preview [get]
func _swaggerPreviewArtifact(c *gin.Context) {}

// @Summary List user conversations
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} conversationsResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/conversations [get]
func _swaggerListConversations(c *gin.Context) {}

// @Summary Get a conversation
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Success 200 {object} model.Conversation
// @Failure 404 {object} map[string]string
// @Router /api/v1/conversations/{id} [get]
func _swaggerGetConversation(c *gin.Context) {}

// @Summary Update conversation title or status
// @Tags agent
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Param request body map[string]string true "Fields to update"
// @Success 200 {object} model.Conversation
// @Failure 404 {object} map[string]string
// @Router /api/v1/conversations/{id} [patch]
func _swaggerUpdateConversation(c *gin.Context) {}

// @Summary Update chat session (r3-chat alias)
// @Tags agent
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Chat ID"
// @Param request body map[string]string true "Fields to update"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /api/v1/chat/sessions/{id} [patch]
func _swaggerUpdateChatSession(c *gin.Context) {}

// @Summary Delete a conversation
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/conversations/{id} [delete]
func _swaggerDeleteConversation(c *gin.Context) {}

// @Summary Delete chat session (r3-chat alias)
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param id path string true "Chat ID"
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /api/v1/chat/sessions/{id} [delete]
func _swaggerDeleteChatSession(c *gin.Context) {}

// @Summary List messages in a conversation
// @Tags agent
// @Produce json
// @Security BearerAuth
// @Param id path string true "Conversation ID"
// @Param limit query int false "Limit" default(50)
// @Success 200 {object} messagesResp
// @Failure 404 {object} map[string]string
// @Router /api/v1/conversations/{id}/messages [get]
func _swaggerListMessages(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Provider (admin)
// ---------------------------------------------------------------------------

// @Summary List AI providers
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} providersResp
// @Failure 401 {object} map[string]string
// @Router /api/v1/admin/providers [get]
func _swaggerListProviders(c *gin.Context) {}

// @Summary Create a new AI provider
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body map[string]interface{} true "Provider config"
// @Success 201 {object} provider.AIProvider
// @Failure 400 {object} map[string]string
// @Router /api/v1/admin/providers [post]
func _swaggerCreateProvider(c *gin.Context) {}

// @Summary Get an AI provider
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Success 200 {object} provider.AIProvider
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/providers/{id} [get]
func _swaggerGetProvider(c *gin.Context) {}

// @Summary Update an AI provider
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Param request body map[string]interface{} true "Provider update"
// @Success 200 {object} provider.AIProvider
// @Failure 400 {object} map[string]string
// @Router /api/v1/admin/providers/{id} [patch]
func _swaggerUpdateProvider(c *gin.Context) {}

// @Summary Delete an AI provider
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/providers/{id} [delete]
func _swaggerDeleteProvider(c *gin.Context) {}

// @Summary Test provider connectivity
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Success 200 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /api/v1/admin/providers/{id}/test [post]
func _swaggerTestProvider(c *gin.Context) {}

// @Summary Sync models from provider
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Success 200 {object} map[string]interface{}
// @Failure 500 {object} map[string]string
// @Router /api/v1/admin/providers/{id}/sync-models [post]
func _swaggerSyncModels(c *gin.Context) {}

// @Summary List models for a provider
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Success 200 {object} providerModelsResp
// @Router /api/v1/admin/providers/{id}/models [get]
func _swaggerListProviderModels(c *gin.Context) {}

// @Summary Create a model for a provider
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Provider ID"
// @Param request body map[string]interface{} true "Model config"
// @Success 201 {object} provider.AIModel
// @Router /api/v1/admin/providers/{id}/models [post]
func _swaggerCreateProviderModel(c *gin.Context) {}

// @Summary Get a model
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Model ID"
// @Success 200 {object} provider.AIModel
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/models/{id} [get]
func _swaggerGetModel(c *gin.Context) {}

// @Summary Update a model
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Model ID"
// @Param request body map[string]interface{} true "Model update"
// @Success 200 {object} provider.AIModel
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/models/{id} [patch]
func _swaggerUpdateModel(c *gin.Context) {}

// @Summary Delete a model
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "Model ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/admin/models/{id} [delete]
func _swaggerDeleteModel(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Billing
// ---------------------------------------------------------------------------

// @Summary List billing plans
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Success 200 {object} plansResp
// @Router /api/v1/billing/plans [get]
func _swaggerListPlans(c *gin.Context) {}

// @Summary Create Stripe checkout session
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body subscribeReq true "Subscription request"
// @Success 200 {object} subscribeResp
// @Failure 400 {object} map[string]string
// @Router /api/v1/billing/subscribe [post]
func _swaggerSubscribe(c *gin.Context) {}

// @Summary Get current subscription
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 200 {object} subscriptionResp
// @Router /api/v1/billing/subscription [get]
func _swaggerGetSubscription(c *gin.Context) {}

// @Summary Cancel subscription
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 200 {object} cancelResp
// @Router /api/v1/billing/cancel [post]
func _swaggerCancelSubscription(c *gin.Context) {}

// @Summary Stripe webhook
// @Tags billing
// @Accept json
// @Produce json
// @Param Stripe-Signature header string true "Stripe signature"
// @Param payload body string true "Webhook payload"
// @Success 200 {object} webhookResp
// @Failure 400 {object} map[string]string
// @Router /api/v1/billing/webhook [post]
func _swaggerStripeWebhook(c *gin.Context) {}

// @Summary Get Stripe subscription (r3-chat)
// @Tags billing
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/stripe/subscription [get]
func _swaggerStripeSubscription(c *gin.Context) {}

// @Summary Create Stripe checkout session (r3-chat)
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body map[string]string true "Price ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/stripe/create-checkout-session [post]
func _swaggerStripeCreateCheckout(c *gin.Context) {}

// @Summary Create Stripe portal session (r3-chat)
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/stripe/create-portal-session [post]
func _swaggerStripeCreatePortal(c *gin.Context) {}

// @Summary Confirm Stripe checkout session (r3-chat)
// @Tags billing
// @Accept json
// @Produce json
// @Param request body map[string]string true "Session ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/stripe/confirm-session [post]
func _swaggerStripeConfirmSession(c *gin.Context) {}

// @Summary Create subscription (r3-chat)
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param request body map[string]string true "Plan"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/subscriptions [post]
func _swaggerCreateSubscription(c *gin.Context) {}

// @Summary Cancel subscription (r3-chat)
// @Tags billing
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/subscriptions/cancel [post]
func _swaggerCancelSubscriptionFrontend(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Usage
// ---------------------------------------------------------------------------

// @Summary Get usage statistics
// @Tags usage
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Success 200 {object} usageStatsResp
// @Router /api/v1/usage/stats [get]
func _swaggerUsageStats(c *gin.Context) {}

// @Summary Get usage limits by plan
// @Tags usage
// @Produce json
// @Success 200 {object} usageLimitsResp
// @Router /api/v1/usage/limits [get]
func _swaggerUsageLimits(c *gin.Context) {}

// @Summary Track a usage event
// @Tags usage
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body trackUsageReq true "Usage event"
// @Success 201 {object} trackUsageResp
// @Router /api/v1/usage/track [post]
func _swaggerTrackUsage(c *gin.Context) {}

// ---------------------------------------------------------------------------
// API Keys
// ---------------------------------------------------------------------------

// @Summary Create a new API key
// @Tags api-keys
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body apiKeyCreateReq true "API key name"
// @Success 201 {object} apiKeyCreateResp
// @Router /api/v1/api-keys [post]
func _swaggerCreateAPIKey(c *gin.Context) {}

// @Summary List API keys
// @Tags api-keys
// @Produce json
// @Security BearerAuth
// @Success 200 {object} apiKeysResp
// @Router /api/v1/api-keys [get]
func _swaggerListAPIKeys(c *gin.Context) {}

// @Summary Revoke an API key
// @Tags api-keys
// @Produce json
// @Security BearerAuth
// @Param id path string true "API Key ID"
// @Success 200 {object} map[string]string
// @Router /api/v1/api-keys/{id} [delete]
func _swaggerRevokeAPIKey(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Files
// ---------------------------------------------------------------------------

// @Summary Upload a file
// @Tags files
// @Accept multipart/form-data
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param file formData file true "File to upload"
// @Success 201 {object} fileupload.Upload
// @Failure 400 {object} map[string]string
// @Router /api/v1/files [post]
func _swaggerUploadFile(c *gin.Context) {}

// @Summary List uploaded files
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param X-User-ID header string true "User ID"
// @Param limit query int false "Limit" default(20)
// @Param offset query int false "Offset" default(0)
// @Success 200 {object} filesResp
// @Router /api/v1/files [get]
func _swaggerListFiles(c *gin.Context) {}

// @Summary Get file metadata
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param id path string true "File ID"
// @Success 200 {object} fileupload.Upload
// @Failure 404 {object} map[string]string
// @Router /api/v1/files/{id} [get]
func _swaggerGetFile(c *gin.Context) {}

// @Summary Download a file
// @Tags files
// @Produce octet-stream
// @Security BearerAuth
// @Param id path string true "File ID"
// @Success 200 {file} binary "File content"
// @Failure 404 {object} map[string]string
// @Router /api/v1/files/{id}/download [get]
func _swaggerDownloadFile(c *gin.Context) {}

// @Summary Delete a file
// @Tags files
// @Produce json
// @Security BearerAuth
// @Param id path string true "File ID"
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/v1/files/{id} [delete]
func _swaggerDeleteFile(c *gin.Context) {}

// ---------------------------------------------------------------------------
// Health
// ---------------------------------------------------------------------------

// @Summary Health check
// @Tags health
// @Produce json
// @Success 200 {object} healthResp
// @Failure 503 {object} healthResp
// @Router /api/v1/health [get]
func _swaggerHealth(c *gin.Context) {}
