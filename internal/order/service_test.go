package order

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

// ---- fakes: the order module is tested without MySQL, the menu or the payment module ----

type fakeRepo struct {
	orders     map[int64]*Order
	nextID     int64
	created    []NewOrder
	dupCodes   int // how many Create calls fail with a code clash first
	transition func(from, to Status) bool
}

func newFakeRepo() *fakeRepo { return &fakeRepo{orders: map[int64]*Order{}, nextID: 1} }

func (f *fakeRepo) Create(_ context.Context, o NewOrder) (int64, error) {
	if f.dupCodes > 0 {
		f.dupCodes--
		return 0, errDuplicateCode
	}
	id := f.nextID
	f.nextID++
	f.created = append(f.created, o)
	f.orders[id] = &Order{ID: id, CanteenID: o.CanteenID, Code: o.Code, Status: PendingPayment, TotalPaise: o.TotalPaise, CustomerPhone: o.Phone, Items: o.Lines}
	return id, nil
}

func (f *fakeRepo) ByID(_ context.Context, id int64) (*Order, error) {
	if o, ok := f.orders[id]; ok {
		c := *o
		return &c, nil
	}
	return nil, apperr.NotFound("order not found")
}

func (f *fakeRepo) ByCodeAndPhone(_ context.Context, canteenID int64, code, phone string) (*Order, error) {
	for _, o := range f.orders {
		if o.CanteenID == canteenID && o.Code == code && o.CustomerPhone == phone {
			c := *o
			return &c, nil
		}
	}
	return nil, apperr.NotFound("order not found")
}

func (f *fakeRepo) List(context.Context, int64, []Status, bool) ([]Order, error) { return nil, nil }

func (f *fakeRepo) Transition(_ context.Context, canteenID, id int64, from, to Status) (bool, error) {
	if f.transition != nil && !f.transition(from, to) {
		return false, nil
	}
	o := f.orders[id]
	if o == nil || o.CanteenID != canteenID || o.Status != from {
		return false, nil
	}
	o.Status = to
	return true, nil
}

func (f *fakeRepo) AdvanceSystem(_ context.Context, id int64, from, to Status) (bool, error) {
	o := f.orders[id]
	if o == nil || o.Status != from {
		return false, nil
	}
	o.Status = to
	return true, nil
}

type fakeCanteens struct{}

func (fakeCanteens) IDBySlug(_ context.Context, slug string) (int64, error) {
	if slug == "main" {
		return 7, nil
	}
	return 0, apperr.NotFound("canteen not found")
}

// fakePricer: item 1 costs 5000; variant 11 costs 9000; add-on 21 costs +1500. Anything else is unknown.
type fakePricer struct{}

func (fakePricer) PriceLine(_ context.Context, canteenID, itemID, variantID int64, addonIDs []int64) (PricedLine, error) {
	if itemID != 1 {
		return PricedLine{}, apperr.Invalid("item %d is not on this canteen's menu", itemID)
	}
	l := PricedLine{ItemID: 1, ItemName: "Pizza", UnitPricePaise: 5000}
	if variantID == 11 {
		l.VariantID, l.VariantName, l.UnitPricePaise = 11, "Large", 9000
	}
	for _, a := range addonIDs {
		if a == 21 {
			l.Addons = append(l.Addons, PricedAddon{ID: 21, Name: "Cheese", PricePaise: 1500})
			l.UnitPricePaise += 1500
		}
	}
	return l, nil
}

type fakePayments struct {
	err    error
	called []int64 // amounts
}

func (p *fakePayments) Initiate(_ context.Context, orderID, amount int64) (string, error) {
	p.called = append(p.called, amount)
	if p.err != nil {
		return "", p.err
	}
	return "gw_order_1", nil
}

func newTestService() (*Service, *fakeRepo, *fakePayments) {
	repo, pay := newFakeRepo(), &fakePayments{}
	s := NewService(repo, fakeCanteens{}, fakePricer{}, pay)
	n := 0
	s.newCode = func() string { n++; return "CODE" + string(rune('A'+n)) }
	return s, repo, pay
}

func validInput() PlaceInput {
	return PlaceInput{
		CustomerName: " Asha ", CustomerPhone: "+91 98765-43210",
		Items: []LineInput{{ItemID: 1, VariantID: 11, AddonIDs: []int64{21}, Quantity: 2}, {ItemID: 1, Quantity: 1}},
	}
}

// ---- tests ----

func TestPlacePricesOnTheServer(t *testing.T) {
	s, repo, pay := newTestService()
	placed, err := s.Place(context.Background(), "main", validInput())
	if err != nil {
		t.Fatal(err)
	}
	// 2 x (9000 + 1500) + 1 x 5000
	const want = 2*10500 + 5000
	if placed.TotalPaise != want || len(pay.called) != 1 || pay.called[0] != want {
		t.Errorf("total=%d paymentAsked=%v, want both %d", placed.TotalPaise, pay.called, want)
	}
	if placed.GatewayOrderID != "gw_order_1" || placed.Code == "" {
		t.Errorf("unexpected response: %+v", placed)
	}
	got := repo.created[0]
	if got.Name != "Asha" || got.Phone != "9876543210" || got.CanteenID != 7 {
		t.Errorf("customer or canteen not normalised: %+v", got)
	}
	if got.Lines[0].VariantName != "Large" || got.Lines[0].Addons[0].Name != "Cheese" {
		t.Errorf("order lines must snapshot names: %+v", got.Lines[0])
	}
	if repo.orders[placed.OrderID].Status != PendingPayment {
		t.Error("a new order must wait for payment")
	}
}

func TestPlaceValidation(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*PlaceInput)
		want   string
	}{
		{"no name", func(i *PlaceInput) { i.CustomerName = "  " }, "customerName"},
		{"short phone", func(i *PlaceInput) { i.CustomerPhone = "12345" }, "10-digit"},
		{"bad email", func(i *PlaceInput) { i.CustomerEmail = "nope" }, "email"},
		{"no items", func(i *PlaceInput) { i.Items = nil }, "between 1 and"},
		{"zero quantity", func(i *PlaceInput) { i.Items[0].Quantity = 0 }, "quantity"},
		{"huge quantity", func(i *PlaceInput) { i.Items[0].Quantity = 1000 }, "quantity"},
		{"unknown item", func(i *PlaceInput) { i.Items[0].ItemID = 99 }, "not on this canteen"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, repo, pay := newTestService()
			in := validInput()
			tc.mutate(&in)
			_, err := s.Place(context.Background(), "main", in)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want error containing %q, got %v", tc.want, err)
			}
			if len(repo.created) != 0 || len(pay.called) != 0 {
				t.Error("nothing may be stored or charged for an invalid order")
			}
		})
	}
}

func TestPlaceUnknownCanteen(t *testing.T) {
	s, _, _ := newTestService()
	if _, err := s.Place(context.Background(), "nope", validInput()); err == nil {
		t.Fatal("want an error for an unknown canteen")
	}
}

func TestPlaceCancelsOrderWhenPaymentSetupFails(t *testing.T) {
	s, repo, pay := newTestService()
	pay.err = apperr.Unavailable("gateway down")
	if _, err := s.Place(context.Background(), "main", validInput()); err == nil {
		t.Fatal("want the payment error")
	}
	if repo.orders[1].Status != Cancelled {
		t.Errorf("order status = %s, want CANCELLED so it is not left pending", repo.orders[1].Status)
	}
}

func TestPlaceRetriesPickupCodeClash(t *testing.T) {
	s, repo, _ := newTestService()
	repo.dupCodes = 2
	if _, err := s.Place(context.Background(), "main", validInput()); err != nil {
		t.Fatalf("two clashes should be retried: %v", err)
	}
	repo.dupCodes = codeTries
	if _, err := s.Place(context.Background(), "main", validInput()); err == nil {
		t.Fatal("persistent clashes must eventually fail")
	}
}

func TestMarkPaidIsIdempotent(t *testing.T) {
	s, repo, _ := newTestService()
	placed, _ := s.Place(context.Background(), "main", validInput())
	for i := 0; i < 2; i++ {
		if err := s.MarkPaid(context.Background(), placed.OrderID); err != nil {
			t.Fatal(err)
		}
	}
	if repo.orders[placed.OrderID].Status != Placed {
		t.Errorf("status = %s, want PLACED", repo.orders[placed.OrderID].Status)
	}
	// a late duplicate must not drag an order that has moved on back to PLACED
	repo.orders[placed.OrderID].Status = Preparing
	if err := s.MarkPaid(context.Background(), placed.OrderID); err != nil || repo.orders[placed.OrderID].Status != Preparing {
		t.Errorf("MarkPaid must leave a PREPARING order alone, got %v / %s", err, repo.orders[placed.OrderID].Status)
	}
	if err := s.MarkPaid(context.Background(), 999); err == nil {
		t.Error("an unknown order must be an error")
	}
}

func TestTrackNeedsCodeAndPhone(t *testing.T) {
	s, _, _ := newTestService()
	placed, _ := s.Place(context.Background(), "main", validInput())
	if o, err := s.Track(context.Background(), "main", strings.ToLower(placed.Code), "98765 43210"); err != nil || o.ID != placed.OrderID {
		t.Fatalf("code (any case) + phone (any format) should match: %v", err)
	}
	if _, err := s.Track(context.Background(), "main", placed.Code, "9999999999"); err == nil {
		t.Error("right code with the wrong phone must not match")
	}
}

func TestCanMove(t *testing.T) {
	allowed := [][2]Status{
		{PendingPayment, Placed}, {PendingPayment, Cancelled}, {Placed, Preparing}, {Placed, Rejected},
		{Preparing, Ready}, {Preparing, Rejected}, {Ready, Completed},
	}
	ok := map[[2]Status]bool{}
	for _, p := range allowed {
		ok[p] = true
	}
	all := []Status{PendingPayment, Placed, Preparing, Ready, Completed, Rejected, Cancelled}
	for _, from := range all {
		for _, to := range all {
			if got := canMove(from, to); got != ok[[2]Status{from, to}] {
				t.Errorf("canMove(%s,%s) = %v, want %v", from, to, got, !got)
			}
		}
	}
}

func TestSetStatus(t *testing.T) {
	ctx := context.Background()
	newPaid := func() (*Service, *fakeRepo, int64) {
		s, repo, _ := newTestService()
		placed, _ := s.Place(ctx, "main", validInput())
		_ = s.MarkPaid(ctx, placed.OrderID)
		return s, repo, placed.OrderID
	}

	t.Run("happy path through the lifecycle", func(t *testing.T) {
		s, _, id := newPaid()
		for _, to := range []Status{Preparing, Ready, Completed} {
			o, err := s.SetStatus(ctx, 7, id, to)
			if err != nil || o.Status != to {
				t.Fatalf("-> %s: %v", to, err)
			}
		}
	})
	t.Run("cannot skip a step", func(t *testing.T) {
		s, _, id := newPaid()
		if _, err := s.SetStatus(ctx, 7, id, Ready); err == nil {
			t.Error("PLACED -> READY must be refused")
		}
	})
	t.Run("admins cannot set system statuses", func(t *testing.T) {
		s, _, id := newPaid()
		for _, to := range []Status{Placed, PendingPayment, Cancelled} {
			if _, err := s.SetStatus(ctx, 7, id, to); err == nil {
				t.Errorf("admin setting %s must be refused", to)
			}
		}
	})
	t.Run("another canteen sees not found", func(t *testing.T) {
		s, _, id := newPaid()
		_, err := s.SetStatus(ctx, 8, id, Preparing)
		if k, _ := apperr.KindOf(err); k != apperr.KindNotFound {
			t.Errorf("want NotFound, got %v", err)
		}
	})
	t.Run("lost race is a conflict", func(t *testing.T) {
		s, repo, id := newPaid()
		repo.transition = func(Status, Status) bool { return false } // someone else changed it first
		_, err := s.SetStatus(ctx, 7, id, Preparing)
		var e *apperr.Error
		if !errors.As(err, &e) || e.Kind != apperr.KindConflict {
			t.Errorf("want Conflict, got %v", err)
		}
	})
}

func TestNormalizePhone(t *testing.T) {
	for in, want := range map[string]string{
		"9876543210": "9876543210", "+91 98765-43210": "9876543210", "(098) 765 43210": "9876543210", "abc": "",
	} {
		if got := normalizePhone(in); got != want {
			t.Errorf("normalizePhone(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRandomCode(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		c := randomCode()
		if len(c) != codeLen || strings.Trim(c, codeAlphabet) != "" {
			t.Fatalf("bad code %q", c)
		}
		seen[c] = true
	}
	if len(seen) < 150 {
		t.Errorf("codes are not random enough: %d distinct of 200", len(seen))
	}
}
