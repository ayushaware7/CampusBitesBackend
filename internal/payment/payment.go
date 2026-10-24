// Package payment owns payments and the Razorpay integration. It never imports the order module:
// it only knows an order by id and reports "this order is paid" through OrderNotifier.
package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

const (
	StatusCreated = "CREATED"
	StatusPaid    = "PAID"
	minAmount     = 100 // paise: the gateway's minimum is Rs 1
)

type Payment struct {
	ID                int64
	OrderID           int64
	ProviderOrderID   string
	ProviderPaymentID string // empty until paid
	AmountPaise       int64
	Status            string
}

// Gateway creates payment orders at the provider (Razorpay in production, a fake in tests).
type Gateway interface {
	CreateOrder(ctx context.Context, amountPaise int64, receipt string) (providerOrderID string, err error)
}

// OrderNotifier is how the order module hears that an order has been paid. Today it is an
// in-process call; if payments become a separate service this becomes an event or HTTP call.
type OrderNotifier interface {
	MarkPaid(ctx context.Context, orderID int64) error
}

// NotifierFunc adapts a function to OrderNotifier (used to break the construction cycle in main).
type NotifierFunc func(ctx context.Context, orderID int64) error

func (f NotifierFunc) MarkPaid(ctx context.Context, orderID int64) error { return f(ctx, orderID) }

type Repository interface {
	Create(ctx context.Context, p Payment) error
	ByOrderID(ctx context.Context, orderID int64) (*Payment, error)
	ByProviderOrderID(ctx context.Context, providerOrderID string) (*Payment, error)
	// MarkPaid records the provider payment id. Idempotent for the same payment id; an error
	// (apperr.Invalid) if the payment was already settled by a different one.
	MarkPaid(ctx context.Context, id int64, providerPaymentID string) error
}

type Service struct {
	repo          Repository
	gateway       Gateway
	notifier      OrderNotifier
	keySecret     string
	webhookSecret string
}

func NewService(repo Repository, gateway Gateway, notifier OrderNotifier, keySecret, webhookSecret string) *Service {
	return &Service{repo, gateway, notifier, keySecret, webhookSecret}
}

func hmacHex(secret string, parts ...[]byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	for _, p := range parts {
		mac.Write(p)
	}
	return hex.EncodeToString(mac.Sum(nil))
}

// validSignature checks the checkout signature: HMAC-SHA256(provider_order_id|payment_id).
func validSignature(secret, providerOrderID, paymentID, signature string) bool {
	return hmac.Equal([]byte(hmacHex(secret, []byte(providerOrderID+"|"+paymentID))), []byte(signature))
}

// validWebhookSignature checks a webhook: HMAC-SHA256 of the raw request body.
func validWebhookSignature(secret string, body []byte, signature string) bool {
	return hmac.Equal([]byte(hmacHex(secret, body)), []byte(signature))
}

// Initiate opens a payment for an order and returns the provider's order id for the checkout popup.
func (s *Service) Initiate(ctx context.Context, orderID, amountPaise int64) (string, error) {
	if amountPaise < minAmount {
		return "", apperr.Invalid("the minimum payment is Rs 1")
	}
	providerID, err := s.gateway.CreateOrder(ctx, amountPaise, fmt.Sprintf("order_%d", orderID))
	if err != nil {
		log.Printf("payment gateway: create order for order %d failed: %v", orderID, err)
		return "", apperr.Unavailable("the payment service is unavailable, please try again")
	}
	err = s.repo.Create(ctx, Payment{OrderID: orderID, ProviderOrderID: providerID, AmountPaise: amountPaise, Status: StatusCreated})
	if err != nil {
		return "", err
	}
	return providerID, nil
}

// Confirm is called by the checkout page after the customer pays. The provider order id comes
// from our own table, never from the request, so a signature for someone else's order is useless.
func (s *Service) Confirm(ctx context.Context, orderID int64, providerPaymentID, signature string) error {
	p, err := s.repo.ByOrderID(ctx, orderID)
	if err != nil {
		return err
	}
	if providerPaymentID == "" || !validSignature(s.keySecret, p.ProviderOrderID, providerPaymentID, signature) {
		return apperr.Invalid("payment verification failed")
	}
	return s.settle(ctx, p, providerPaymentID)
}

// settle records the payment and tells the order module. Both steps are safe to repeat, so a
// retry (or the webhook arriving later) heals a failure between the two.
func (s *Service) settle(ctx context.Context, p *Payment, providerPaymentID string) error {
	if err := s.repo.MarkPaid(ctx, p.ID, providerPaymentID); err != nil {
		return err
	}
	if err := s.notifier.MarkPaid(ctx, p.OrderID); err != nil {
		log.Printf("CRITICAL: payment %s recorded but order %d could not be marked paid: %v", providerPaymentID, p.OrderID, err)
		return err
	}
	return nil
}

// HandleWebhook processes Razorpay's payment.captured event, the safety net for a customer who
// paid but closed the browser before Confirm ran. Disabled unless a webhook secret is configured.
func (s *Service) HandleWebhook(ctx context.Context, body []byte, signature string) error {
	if s.webhookSecret == "" {
		return apperr.NotFound("webhooks are not enabled")
	}
	if !validWebhookSignature(s.webhookSecret, body, signature) {
		return apperr.Unauthorized("invalid webhook signature")
	}
	var ev struct {
		Event   string `json:"event"`
		Payload struct {
			Payment struct {
				Entity struct {
					ID      string `json:"id"`
					OrderID string `json:"order_id"`
				} `json:"entity"`
			} `json:"payment"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &ev); err != nil {
		return apperr.Invalid("invalid webhook body")
	}
	if ev.Event != "payment.captured" {
		return nil // other events are acknowledged and ignored
	}
	p, err := s.repo.ByProviderOrderID(ctx, ev.Payload.Payment.Entity.OrderID)
	if kind, ok := apperr.KindOf(err); ok && kind == apperr.KindNotFound {
		return nil // not one of ours
	}
	if err != nil {
		return err
	}
	return s.settle(ctx, p, ev.Payload.Payment.Entity.ID)
}
