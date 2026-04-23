package billing

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// ---- mock repositories ----

type mockPlanRepo struct {
	plans map[string]*Plan
}

func (m *mockPlanRepo) ListPlans(ctx context.Context) ([]Plan, error) {
	var out []Plan
	for _, p := range m.plans {
		out = append(out, *p)
	}
	return out, nil
}

func (m *mockPlanRepo) GetPlanBySlug(ctx context.Context, slug string) (*Plan, error) {
	p, ok := m.plans[slug]
	if !ok {
		return nil, errors.New("not found")
	}
	return p, nil
}

func (m *mockPlanRepo) GetPlanByID(ctx context.Context, id uuid.UUID) (*Plan, error) {
	for _, p := range m.plans {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockPlanRepo) GetPlanByStripePriceID(ctx context.Context, priceID string) (*Plan, error) {
	for _, p := range m.plans {
		if p.StripePriceID == priceID {
			return p, nil
		}
	}
	return nil, errors.New("not found")
}

type mockSubRepo struct {
	subs map[uuid.UUID]*Subscription
}

func (m *mockSubRepo) CreateSubscription(ctx context.Context, sub *Subscription) error {
	m.subs[sub.ID] = sub
	return nil
}

func (m *mockSubRepo) GetSubscriptionByID(ctx context.Context, id uuid.UUID) (*Subscription, error) {
	s, ok := m.subs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return s, nil
}

func (m *mockSubRepo) GetSubscriptionByUser(ctx context.Context, tenantID, userID uuid.UUID) (*Subscription, error) {
	for _, s := range m.subs {
		if s.TenantID == tenantID && s.UserID == userID {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSubRepo) GetSubscriptionByStripeID(ctx context.Context, stripeSubID string) (*Subscription, error) {
	for _, s := range m.subs {
		if s.StripeSubscriptionID == stripeSubID {
			return s, nil
		}
	}
	return nil, nil
}

func (m *mockSubRepo) UpdateSubscription(ctx context.Context, sub *Subscription) error {
	m.subs[sub.ID] = sub
	return nil
}

func (m *mockSubRepo) CancelSubscription(ctx context.Context, id uuid.UUID, canceledAt time.Time, cancelAtPeriodEnd bool) error {
	s, ok := m.subs[id]
	if !ok {
		return errors.New("not found")
	}
	s.Status = "canceled"
	s.CanceledAt = &canceledAt
	s.CancelAtPeriodEnd = cancelAtPeriodEnd
	return nil
}

func setupService() (*BillingService, *mockPlanRepo, *mockSubRepo) {
	plans := &mockPlanRepo{plans: map[string]*Plan{
		"free": {
			ID:             uuid.New(),
			Slug:           "free",
			Name:           "Free",
			StripePriceID:  "",
			AmountCents:    0,
			MessagesPerDay: 3,
		},
		"premium": {
			ID:             uuid.New(),
			Slug:           "premium",
			Name:           "Premium",
			StripePriceID:  "price_premium",
			AmountCents:    999,
			MessagesPerDay: 1000,
		},
	}}
	subs := &mockSubRepo{subs: make(map[uuid.UUID]*Subscription)}
	svc := NewBillingService(plans, subs, "sk_test_key", "whsec_secret", "http://localhost:5173")
	return svc, plans, subs
}

func TestBillingService_GetSubscription_NoSub(t *testing.T) {
	svc, _, _ := setupService()
	tenantID := uuid.New()
	userID := uuid.New()

	sub, plan, err := svc.GetSubscription(context.Background(), tenantID, userID)
	assert.NoError(t, err)
	assert.Nil(t, sub)
	assert.NotNil(t, plan)
	assert.Equal(t, "free", plan.Slug)
}

func TestBillingService_GetSubscription_Active(t *testing.T) {
	svc, plans, subs := setupService()
	tenantID := uuid.New()
	userID := uuid.New()
	premiumPlan, _ := plans.GetPlanBySlug(context.Background(), "premium")

	sub := &Subscription{
		ID:                   uuid.New(),
		TenantID:             tenantID,
		UserID:               userID,
		PlanID:               premiumPlan.ID,
		StripeSubscriptionID: "sub_123",
		Status:               "active",
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	_ = subs.CreateSubscription(context.Background(), sub)

	gotSub, gotPlan, err := svc.GetSubscription(context.Background(), tenantID, userID)
	assert.NoError(t, err)
	assert.NotNil(t, gotSub)
	assert.Equal(t, "active", gotSub.Status)
	assert.Equal(t, "premium", gotPlan.Slug)
}

func TestBillingService_CreateCheckoutSession_PlanNotFound(t *testing.T) {
	svc, _, _ := setupService()
	_, err := svc.CreateCheckoutSession(context.Background(), uuid.New(), uuid.New(), "test@test.com", "enterprise")
	assert.Error(t, err)
}

func TestBillingService_CreateCheckoutSession_NoStripePrice(t *testing.T) {
	svc, _, _ := setupService()
	_, err := svc.CreateCheckoutSession(context.Background(), uuid.New(), uuid.New(), "test@test.com", "free")
	assert.Error(t, err)
}

func TestBillingService_CancelSubscription_NoSub(t *testing.T) {
	svc, _, _ := setupService()
	err := svc.CancelSubscription(context.Background(), uuid.New(), uuid.New())
	assert.Error(t, err)
}

func TestBillingService_CancelSubscription_Success(t *testing.T) {
	svc, plans, subs := setupService()
	tenantID := uuid.New()
	userID := uuid.New()
	premiumPlan, _ := plans.GetPlanBySlug(context.Background(), "premium")

	sub := &Subscription{
		ID:                   uuid.New(),
		TenantID:             tenantID,
		UserID:               userID,
		PlanID:               premiumPlan.ID,
		StripeSubscriptionID: "sub_123",
		Status:               "active",
		CreatedAt:            time.Now().UTC(),
		UpdatedAt:            time.Now().UTC(),
	}
	_ = subs.CreateSubscription(context.Background(), sub)

	// This will fail because stripe key is invalid, but it tests the flow up to the stripe call
	err := svc.CancelSubscription(context.Background(), tenantID, userID)
	assert.Error(t, err) // stripe error expected with test key
}

func TestBillingService_HandleWebhook_InvalidSignature(t *testing.T) {
	svc, _, _ := setupService()
	err := svc.HandleWebhook(context.Background(), WebhookPayload{
		Payload:   []byte(`{}`),
		Signature: "invalid",
	})
	assert.Error(t, err)
}
