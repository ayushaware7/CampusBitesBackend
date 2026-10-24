// Package order owns orders and their lifecycle. It never imports the menu or payment modules:
// it talks to them through the small interfaces in service.go, which cmd/server wires up.
package order

import "time"

type Status string

const (
	PendingPayment Status = "PENDING_PAYMENT" // created, waiting for the customer to pay
	Placed         Status = "PLACED"          // paid; the canteen has not started yet
	Preparing      Status = "PREPARING"
	Ready          Status = "READY" // ready for pickup
	Completed      Status = "COMPLETED"
	Rejected       Status = "REJECTED"
	Cancelled      Status = "CANCELLED" // never paid (system)
)

// transitions is the whole lifecycle: anything not listed here is refused.
var transitions = map[Status][]Status{
	PendingPayment: {Placed, Cancelled},
	Placed:         {Preparing, Rejected},
	Preparing:      {Ready, Rejected},
	Ready:          {Completed},
}

func canMove(from, to Status) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// adminTargets are the statuses a canteen admin may set; the others are set by the system.
var adminTargets = map[Status]bool{Preparing: true, Ready: true, Completed: true, Rejected: true}

var (
	liveStatuses = []Status{Placed, Preparing, Ready}
	pastStatuses = []Status{Completed, Rejected}
)

type AddonLine struct {
	AddonID    int64  `json:"-"`
	Name       string `json:"name"`
	PricePaise int64  `json:"pricePaise"`
}

// Line is one purchased line, as it was at order time.
type Line struct {
	ItemID         int64       `json:"-"`
	VariantID      int64       `json:"-"`
	ItemName       string      `json:"itemName"`
	VariantName    string      `json:"variantName"`
	Quantity       int         `json:"quantity"`
	UnitPricePaise int64       `json:"unitPricePaise"`
	Addons         []AddonLine `json:"addons"`
}

type Order struct {
	ID            int64     `json:"id"`
	CanteenID     int64     `json:"-"`
	Code          string    `json:"code"`
	Status        Status    `json:"status"`
	CustomerName  string    `json:"customerName"`
	CustomerPhone string    `json:"customerPhone"`
	CustomerEmail string    `json:"customerEmail"`
	TotalPaise    int64     `json:"totalPaise"`
	CreatedAt     time.Time `json:"createdAt"`
	Items         []Line    `json:"items"`
}

// ---- request / response shapes ----

type LineInput struct {
	ItemID    int64   `json:"itemId"`
	VariantID int64   `json:"variantId"`
	AddonIDs  []int64 `json:"addonIds"`
	Quantity  int     `json:"quantity"`
}

type PlaceInput struct {
	CustomerName  string      `json:"customerName"`
	CustomerPhone string      `json:"customerPhone"`
	CustomerEmail string      `json:"customerEmail"`
	Items         []LineInput `json:"items"`
}

// PlacedOrder is what the checkout page needs to open the payment popup.
type PlacedOrder struct {
	OrderID        int64  `json:"orderId"`
	Code           string `json:"code"`
	TotalPaise     int64  `json:"totalPaise"`
	GatewayOrderID string `json:"gatewayOrderId"`
}

// NewOrder is what the repository stores in one transaction.
type NewOrder struct {
	CanteenID  int64
	Code       string
	Name       string
	Phone      string
	Email      string
	TotalPaise int64
	Lines      []Line
}
