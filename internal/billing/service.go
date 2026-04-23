package billing

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/checkout/session"
	"github.com/stripe/stripe-go/v81/customer"
	"github.com/stripe/stripe-go/v81/subscription"
	"github.com/stripe/stripe-go/v81/webhook"
)

// BillingService handles Stripe integration and subscription logic.
type BillingService struct {
	plans        PlanRepository
	subs         SubscriptionRepository
	stripeKey    string
	webhookSecret string
	frontendURL  string
}

// NewBillingService creates a new billing service.
func NewBillingService(plans PlanRepository, subs SubscriptionRepository, stripeKey, webhookSecret, frontendURL string) *BillingService {
	stripe.Key = stripeKey
	return &BillingService{
		plans:         plans,
		subs:          subs,
		stripeKey:     stripeKey,
		webhookSecret: webhookSecret,
		frontendURL:   frontendURL,
	}
}

// CreateCheckoutSession creates a Stripe checkout session for a plan.
func (s *BillingService) CreateCheckoutSession(ctx context.Context, tenantID, userID uuid.UUID, email, planSlug string) (string, error) {
	plan, err := s.plans.GetPlanBySlug(ctx, planSlug)
	if err != nil {
		return "", fmt.Errorf("plan not found: %w", err)
	}
	if plan.StripePriceID == "" {
		return "", fmt.Errorf("plan %s has no stripe price id", planSlug)
	}

	// Create or reuse Stripe customer
	cusParams := &stripe.CustomerParams{
		Email: stripe.String(email),
		Metadata: map[string]string{
			"tenant_id": tenantID.String(),
			"user_id":   userID.String(),
		},
	}
	cus, err := customer.New(cusParams)
	if err != nil {
		return "", fmt.Errorf("create customer: %w", err)
	}

	params := &stripe.CheckoutSessionParams{
		Customer:   stripe.String(cus.ID),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(plan.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		Mode:       stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		SuccessURL: stripe.String(s.frontendURL + "/billing/success?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(s.frontendURL + "/billing/cancel"),
		SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
			Metadata: map[string]string{
				"tenant_id": tenantID.String(),
				"user_id":   userID.String(),
				"plan_id":   plan.ID.String(),
			},
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	// Pre-create subscription record (incomplete until webhook confirms)
	sub := &Subscription{
		ID:               uuid.New(),
		TenantID:         tenantID,
		UserID:           userID,
		PlanID:           plan.ID,
		StripeCustomerID: cus.ID,
		Status:           "incomplete",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	if err := s.subs.CreateSubscription(ctx, sub); err != nil {
		return "", fmt.Errorf("save subscription: %w", err)
	}

	return sess.URL, nil
}

// GetSubscription returns the current subscription for a user.
func (s *BillingService) GetSubscription(ctx context.Context, tenantID, userID uuid.UUID) (*Subscription, *Plan, error) {
	sub, err := s.subs.GetSubscriptionByUser(ctx, tenantID, userID)
	if err != nil {
		return nil, nil, err
	}
	if sub == nil {
		// Return free plan as default
		plan, err := s.plans.GetPlanBySlug(ctx, "free")
		if err != nil {
			return nil, nil, err
		}
		return nil, plan, nil
	}
	plan, err := s.plans.GetPlanByID(ctx, sub.PlanID)
	if err != nil {
		return nil, nil, err
	}
	return sub, plan, nil
}

// CancelSubscription cancels via Stripe and updates local record.
func (s *BillingService) CancelSubscription(ctx context.Context, tenantID, userID uuid.UUID) error {
	sub, err := s.subs.GetSubscriptionByUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	if sub == nil || sub.StripeSubscriptionID == "" {
		return fmt.Errorf("no active subscription")
	}

	_, err = subscription.Cancel(sub.StripeSubscriptionID, nil)
	if err != nil {
		return fmt.Errorf("stripe cancel: %w", err)
	}

	now := time.Now().UTC()
	return s.subs.CancelSubscription(ctx, sub.ID, now, true)
}

// WebhookPayload holds the raw Stripe event.
type WebhookPayload struct {
	Payload   []byte
	Signature string
}

// HandleWebhook processes Stripe events.
func (s *BillingService) HandleWebhook(ctx context.Context, p WebhookPayload) error {
	event, err := webhook.ConstructEvent(p.Payload, p.Signature, s.webhookSecret)
	if err != nil {
		return fmt.Errorf("invalid webhook signature: %w", err)
	}

	switch event.Type {
	case "checkout.session.completed":
		return s.handleCheckoutCompleted(ctx, event)
	case "invoice.paid":
		return s.handleInvoicePaid(ctx, event)
	case "customer.subscription.updated":
		return s.handleSubscriptionUpdated(ctx, event)
	case "customer.subscription.deleted":
		return s.handleSubscriptionDeleted(ctx, event)
	}
	return nil
}

func (s *BillingService) handleCheckoutCompleted(ctx context.Context, event stripe.Event) error {
	var sess stripe.CheckoutSession
	if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
		return err
	}
	if sess.Subscription == nil {
		return nil
	}

	sub, err := s.subs.GetSubscriptionByStripeID(ctx, sess.Subscription.ID)
	if err != nil {
		return err
	}
	if sub == nil {
		return fmt.Errorf("subscription not found for stripe id %s", sess.Subscription.ID)
	}

	sub.StripeSubscriptionID = sess.Subscription.ID
	sub.Status = string(stripe.SubscriptionStatusActive)
	sub.UpdatedAt = time.Now().UTC()
	return s.subs.UpdateSubscription(ctx, sub)
}

func (s *BillingService) handleInvoicePaid(ctx context.Context, event stripe.Event) error {
	var inv stripe.Invoice
	if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
		return err
	}
	if inv.Subscription == nil {
		return nil
	}

	sub, err := s.subs.GetSubscriptionByStripeID(ctx, inv.Subscription.ID)
	if err != nil {
		return err
	}
	if sub == nil {
		return nil
	}

	sub.Status = string(stripe.SubscriptionStatusActive)
	sub.UpdatedAt = time.Now().UTC()
	return s.subs.UpdateSubscription(ctx, sub)
}

func (s *BillingService) handleSubscriptionUpdated(ctx context.Context, event stripe.Event) error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return err
	}

	sub, err := s.subs.GetSubscriptionByStripeID(ctx, stripeSub.ID)
	if err != nil {
		return err
	}
	if sub == nil {
		return nil
	}

	sub.Status = string(stripeSub.Status)
	if stripeSub.CurrentPeriodStart > 0 {
		t := time.Unix(stripeSub.CurrentPeriodStart, 0).UTC()
		sub.CurrentPeriodStart = &t
	}
	if stripeSub.CurrentPeriodEnd > 0 {
		t := time.Unix(stripeSub.CurrentPeriodEnd, 0).UTC()
		sub.CurrentPeriodEnd = &t
	}
	sub.CancelAtPeriodEnd = stripeSub.CancelAtPeriodEnd
	sub.UpdatedAt = time.Now().UTC()
	return s.subs.UpdateSubscription(ctx, sub)
}

func (s *BillingService) handleSubscriptionDeleted(ctx context.Context, event stripe.Event) error {
	var stripeSub stripe.Subscription
	if err := json.Unmarshal(event.Data.Raw, &stripeSub); err != nil {
		return err
	}

	sub, err := s.subs.GetSubscriptionByStripeID(ctx, stripeSub.ID)
	if err != nil {
		return err
	}
	if sub == nil {
		return nil
	}

	now := time.Now().UTC()
	return s.subs.CancelSubscription(ctx, sub.ID, now, false)
}
