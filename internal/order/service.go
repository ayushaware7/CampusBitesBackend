package order

import (
	"context"
	"crypto/rand"
	"errors"
	"log"
	"strings"
	"unicode/utf8"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

// ---- what this module needs from the others (implemented and wired in cmd/server) ----

// CanteenLookup resolves a QR slug to a canteen id.
type CanteenLookup interface {
	IDBySlug(ctx context.Context, slug string) (int64, error)
}

type PricedAddon struct {
	ID         int64
	Name       string
	PricePaise int64
}

type PricedLine struct {
	ItemID         int64
	ItemName       string
	VariantID      int64
	VariantName    string
	Addons         []PricedAddon
	UnitPricePaise int64
}

// Pricer validates one cart line against the live menu and prices it (the menu module).
type Pricer interface {
	PriceLine(ctx context.Context, canteenID, itemID, variantID int64, addonIDs []int64) (PricedLine, error)
}

// Payments opens a payment for an order and returns the gateway's order id (the payment module).
type Payments interface {
	Initiate(ctx context.Context, orderID, amountPaise int64) (gatewayOrderID string, err error)
}

const (
	maxLines    = 30
	maxQuantity = 50
	codeLen     = 6
	codeTries   = 5
	// no 0/O/1/I so a code read out loud or copied by hand is not ambiguous
	codeAlphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
)

type Service struct {
	repo     Repository
	canteens CanteenLookup
	pricer   Pricer
	payments Payments
	newCode  func() string
}

func NewService(repo Repository, canteens CanteenLookup, pricer Pricer, payments Payments) *Service {
	return &Service{repo, canteens, pricer, payments, randomCode}
}

func randomCode() string {
	b := make([]byte, codeLen)
	if _, err := rand.Read(b); err != nil {
		panic(err) // the OS random source failing is not recoverable
	}
	for i := range b {
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b)
}

// normalizePhone keeps the last 10 digits, so "+91 98765-43210" and "9876543210" are the same customer.
// shortcut: assumes 10-digit (Indian) numbers, change if other countries are served.
func normalizePhone(p string) string {
	var d strings.Builder
	for _, r := range p {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	s := d.String()
	if len(s) > 10 {
		s = s[len(s)-10:]
	}
	return s
}

func validatePlace(in *PlaceInput) error {
	in.CustomerName = strings.TrimSpace(in.CustomerName)
	if in.CustomerName == "" || utf8.RuneCountInString(in.CustomerName) > 100 {
		return apperr.Invalid("customerName is required (max 100 characters)")
	}
	in.CustomerPhone = normalizePhone(in.CustomerPhone)
	if len(in.CustomerPhone) != 10 {
		return apperr.Invalid("customerPhone must be a 10-digit phone number")
	}
	in.CustomerEmail = strings.TrimSpace(in.CustomerEmail)
	if in.CustomerEmail != "" && (!strings.Contains(in.CustomerEmail, "@") || len(in.CustomerEmail) > 100) {
		return apperr.Invalid("customerEmail is not a valid email")
	}
	if len(in.Items) == 0 || len(in.Items) > maxLines {
		return apperr.Invalid("an order needs between 1 and %d items", maxLines)
	}
	for _, l := range in.Items {
		if l.ItemID <= 0 || l.Quantity < 1 || l.Quantity > maxQuantity {
			return apperr.Invalid("each item needs an itemId and a quantity between 1 and %d", maxQuantity)
		}
	}
	return nil
}

// Place prices the cart on the server, stores the order as PENDING_PAYMENT and opens the payment.
// The order exists before the customer pays, so a payment can never be left without an order.
func (s *Service) Place(ctx context.Context, slug string, in PlaceInput) (*PlacedOrder, error) {
	if err := validatePlace(&in); err != nil {
		return nil, err
	}
	canteenID, err := s.canteens.IDBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}

	lines := make([]Line, 0, len(in.Items))
	var total int64
	for _, li := range in.Items {
		p, err := s.pricer.PriceLine(ctx, canteenID, li.ItemID, li.VariantID, li.AddonIDs)
		if err != nil {
			return nil, err
		}
		line := Line{
			ItemID: p.ItemID, VariantID: p.VariantID, ItemName: p.ItemName, VariantName: p.VariantName,
			Quantity: li.Quantity, UnitPricePaise: p.UnitPricePaise, Addons: []AddonLine{},
		}
		for _, a := range p.Addons {
			line.Addons = append(line.Addons, AddonLine{AddonID: a.ID, Name: a.Name, PricePaise: a.PricePaise})
		}
		lines = append(lines, line)
		total += p.UnitPricePaise * int64(li.Quantity)
	}

	var id int64
	var code string
	for try := 0; try < codeTries; try++ {
		code = s.newCode()
		id, err = s.repo.Create(ctx, NewOrder{
			CanteenID: canteenID, Code: code, Name: in.CustomerName, Phone: in.CustomerPhone,
			Email: in.CustomerEmail, TotalPaise: total, Lines: lines,
		})
		if !errors.Is(err, errDuplicateCode) {
			break
		}
	}
	if err != nil {
		return nil, err
	}

	gatewayID, err := s.payments.Initiate(ctx, id, total)
	if err != nil {
		// nothing was charged: close the order so it never shows up as pending forever
		if _, cerr := s.repo.AdvanceSystem(ctx, id, PendingPayment, Cancelled); cerr != nil {
			log.Printf("could not cancel order %d after payment setup failed: %v", id, cerr)
		}
		return nil, err
	}
	return &PlacedOrder{OrderID: id, Code: code, TotalPaise: total, GatewayOrderID: gatewayID}, nil
}

// MarkPaid is called by the payment module once a payment is confirmed. Safe to call twice.
func (s *Service) MarkPaid(ctx context.Context, orderID int64) error {
	o, err := s.repo.ByID(ctx, orderID)
	if err != nil {
		return err
	}
	if o.Status != PendingPayment {
		return nil // already paid (a retry or the webhook arrived first)
	}
	if _, err := s.repo.AdvanceSystem(ctx, orderID, PendingPayment, Placed); err != nil {
		return err
	}
	return nil
}

// Track lets a customer see their order with the pickup code and the phone number they ordered with.
func (s *Service) Track(ctx context.Context, slug, code, phone string) (*Order, error) {
	canteenID, err := s.canteens.IDBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	o, err := s.repo.ByCodeAndPhone(ctx, canteenID, strings.ToUpper(strings.TrimSpace(code)), normalizePhone(phone))
	if kind, ok := apperr.KindOf(err); ok && kind == apperr.KindNotFound {
		return nil, apperr.NotFound("no order matches that code and phone number")
	}
	return o, err
}

func (s *Service) Live(ctx context.Context, canteenID int64) ([]Order, error) {
	return s.repo.List(ctx, canteenID, liveStatuses, false)
}

func (s *Service) Past(ctx context.Context, canteenID int64) ([]Order, error) {
	return s.repo.List(ctx, canteenID, pastStatuses, true)
}

// SetStatus lets a canteen admin move one of its own orders along the lifecycle.
func (s *Service) SetStatus(ctx context.Context, canteenID, id int64, to Status) (*Order, error) {
	if !adminTargets[to] {
		return nil, apperr.Invalid("status must be one of PREPARING, READY, COMPLETED, REJECTED")
	}
	o, err := s.repo.ByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if o.CanteenID != canteenID {
		return nil, apperr.NotFound("order not found") // never reveal other canteens' orders
	}
	if !canMove(o.Status, to) {
		return nil, apperr.Invalid("cannot change an order from %s to %s", o.Status, to)
	}
	ok, err := s.repo.Transition(ctx, canteenID, id, o.Status, to)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, apperr.Conflict("this order was just changed by someone else; refresh and try again")
	}
	return s.repo.ByID(ctx, id)
}
