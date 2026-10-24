package payment

import (
	"context"
	"errors"
	"testing"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

const (
	keySecret     = "key_secret"
	webhookSecret = "hook_secret"
)

// ---- fakes: the payment module is tested without MySQL, Razorpay or the order module ----

type fakeRepo struct{ byOrder map[int64]*Payment }

func (f *fakeRepo) Create(_ context.Context, p Payment) error {
	if _, dup := f.byOrder[p.OrderID]; dup {
		return apperr.Invalid("a payment already exists for this order")
	}
	f.byOrder[p.OrderID] = &p
	return nil
}

func (f *fakeRepo) ByOrderID(_ context.Context, id int64) (*Payment, error) {
	if p, ok := f.byOrder[id]; ok {
		return p, nil
	}
	return nil, apperr.NotFound("payment not found")
}

func (f *fakeRepo) ByProviderOrderID(_ context.Context, id string) (*Payment, error) {
	for _, p := range f.byOrder {
		if p.ProviderOrderID == id {
			return p, nil
		}
	}
	return nil, apperr.NotFound("payment not found")
}

func (f *fakeRepo) MarkPaid(_ context.Context, id int64, payID string) error {
	for _, p := range f.byOrder {
		if p.ID == id {
			if p.ProviderPaymentID != "" && p.ProviderPaymentID != payID {
				return apperr.Invalid("this order was already paid with a different payment")
			}
			p.ProviderPaymentID, p.Status = payID, StatusPaid
		}
	}
	return nil
}

type fakeGateway struct {
	err    error
	amount int64
}

func (g *fakeGateway) CreateOrder(_ context.Context, amount int64, _ string) (string, error) {
	g.amount = amount
	return "order_rzp_1", g.err
}

type fakeNotifier struct {
	paid []int64
	err  error
}

func (n *fakeNotifier) MarkPaid(_ context.Context, id int64) error {
	n.paid = append(n.paid, id)
	return n.err
}

func newTestService(secret string) (*Service, *fakeRepo, *fakeGateway, *fakeNotifier) {
	repo, gw, n := &fakeRepo{byOrder: map[int64]*Payment{}}, &fakeGateway{}, &fakeNotifier{}
	return NewService(repo, gw, n, keySecret, secret), repo, gw, n
}

// ---- tests ----

func TestSignatures(t *testing.T) {
	good := hmacHex(keySecret, []byte("order_rzp_1|pay_9"))
	if !validSignature(keySecret, "order_rzp_1", "pay_9", good) {
		t.Error("a correct signature was rejected")
	}
	for name, ok := range map[string]bool{
		"other payment": validSignature(keySecret, "order_rzp_1", "pay_8", good),
		"other order":   validSignature(keySecret, "order_rzp_2", "pay_9", good),
		"other secret":  validSignature("nope", "order_rzp_1", "pay_9", good),
		"empty":         validSignature(keySecret, "order_rzp_1", "pay_9", ""),
	} {
		if ok {
			t.Errorf("signature accepted for: %s", name)
		}
	}
	body := []byte(`{"event":"payment.captured"}`)
	if !validWebhookSignature(webhookSecret, body, hmacHex(webhookSecret, body)) || validWebhookSignature(webhookSecret, body, "x") {
		t.Error("webhook signature check is wrong")
	}
}

func TestInitiate(t *testing.T) {
	s, repo, gw, _ := newTestService("")
	ctx := context.Background()

	if _, err := s.Initiate(ctx, 1, 50); err == nil {
		t.Error("amounts under Rs 1 must be refused")
	}
	id, err := s.Initiate(ctx, 1, 25000)
	if err != nil || id != "order_rzp_1" || gw.amount != 25000 {
		t.Fatalf("id=%q amount=%d err=%v", id, gw.amount, err)
	}
	if p := repo.byOrder[1]; p == nil || p.Status != StatusCreated || p.AmountPaise != 25000 {
		t.Errorf("payment not stored correctly: %+v", p)
	}

	gw.err = errors.New("connection refused")
	_, err = s.Initiate(ctx, 2, 25000)
	if k, _ := apperr.KindOf(err); k != apperr.KindUnavailable {
		t.Errorf("a gateway failure must surface as Unavailable, got %v", err)
	}
}

func TestConfirm(t *testing.T) {
	ctx := context.Background()
	setup := func() (*Service, *fakeRepo, *fakeNotifier) {
		s, repo, _, n := newTestService("")
		repo.byOrder[5] = &Payment{ID: 1, OrderID: 5, ProviderOrderID: "order_rzp_1", AmountPaise: 25000, Status: StatusCreated}
		return s, repo, n
	}
	sig := func(payID string) string { return hmacHex(keySecret, []byte("order_rzp_1|"+payID)) }

	t.Run("valid payment marks the order paid", func(t *testing.T) {
		s, repo, n := setup()
		if err := s.Confirm(ctx, 5, "pay_1", sig("pay_1")); err != nil {
			t.Fatal(err)
		}
		if repo.byOrder[5].Status != StatusPaid || len(n.paid) != 1 || n.paid[0] != 5 {
			t.Errorf("status=%s notified=%v", repo.byOrder[5].Status, n.paid)
		}
	})
	t.Run("bad signature changes nothing", func(t *testing.T) {
		s, repo, n := setup()
		if err := s.Confirm(ctx, 5, "pay_1", "forged"); err == nil {
			t.Fatal("a forged signature must be refused")
		}
		if repo.byOrder[5].Status != StatusCreated || len(n.paid) != 0 {
			t.Error("a refused payment must not mark anything paid")
		}
	})
	t.Run("signature for another order is useless", func(t *testing.T) {
		s, _, n := setup()
		other := hmacHex(keySecret, []byte("order_rzp_999|pay_1"))
		if err := s.Confirm(ctx, 5, "pay_1", other); err == nil || len(n.paid) != 0 {
			t.Error("the provider order id must come from our own table")
		}
	})
	t.Run("retry is safe and heals a failed notification", func(t *testing.T) {
		s, _, n := setup()
		n.err = errors.New("db down")
		if err := s.Confirm(ctx, 5, "pay_1", sig("pay_1")); err == nil {
			t.Fatal("a failed notification must be reported so the client retries")
		}
		n.err = nil
		if err := s.Confirm(ctx, 5, "pay_1", sig("pay_1")); err != nil || len(n.paid) != 2 {
			t.Errorf("retry should succeed and notify again: err=%v notified=%v", err, n.paid)
		}
	})
	t.Run("a second, different payment is refused", func(t *testing.T) {
		s, _, _ := setup()
		_ = s.Confirm(ctx, 5, "pay_1", sig("pay_1"))
		if err := s.Confirm(ctx, 5, "pay_2", sig("pay_2")); err == nil {
			t.Error("an order must not be settled by two different payments")
		}
	})
	t.Run("unknown order", func(t *testing.T) {
		s, _, _ := setup()
		if err := s.Confirm(ctx, 404, "pay_1", sig("pay_1")); err == nil {
			t.Error("want an error for an order without a payment")
		}
	})
}

func TestHandleWebhook(t *testing.T) {
	ctx := context.Background()
	body := []byte(`{"event":"payment.captured","payload":{"payment":{"entity":{"id":"pay_7","order_id":"order_rzp_1"}}}}`)

	t.Run("disabled without a secret", func(t *testing.T) {
		s, _, _, _ := newTestService("")
		if err := s.HandleWebhook(ctx, body, hmacHex("", body)); err == nil {
			t.Error("webhooks must be off when no secret is configured")
		}
	})
	t.Run("rejects a bad signature", func(t *testing.T) {
		s, _, _, n := newTestService(webhookSecret)
		if err := s.HandleWebhook(ctx, body, "forged"); err == nil || len(n.paid) != 0 {
			t.Error("a forged webhook must be refused")
		}
	})
	t.Run("captured payment settles the order", func(t *testing.T) {
		s, repo, _, n := newTestService(webhookSecret)
		repo.byOrder[5] = &Payment{ID: 1, OrderID: 5, ProviderOrderID: "order_rzp_1", Status: StatusCreated}
		if err := s.HandleWebhook(ctx, body, hmacHex(webhookSecret, body)); err != nil {
			t.Fatal(err)
		}
		if repo.byOrder[5].ProviderPaymentID != "pay_7" || len(n.paid) != 1 {
			t.Errorf("payment=%+v notified=%v", repo.byOrder[5], n.paid)
		}
	})
	t.Run("ignores other events and unknown orders", func(t *testing.T) {
		s, _, _, n := newTestService(webhookSecret)
		other := []byte(`{"event":"payment.failed"}`)
		if err := s.HandleWebhook(ctx, other, hmacHex(webhookSecret, other)); err != nil {
			t.Error(err)
		}
		if err := s.HandleWebhook(ctx, body, hmacHex(webhookSecret, body)); err != nil || len(n.paid) != 0 {
			t.Errorf("an event for an unknown order must be acknowledged and ignored: %v", err)
		}
	})
}
