package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"time"

	"github.com/go-sql-driver/mysql"
)

// Order is one row of live_orders or past_orders; JSON names match what the React app reads.
type Order struct {
	ID                int64      `json:"id"`
	RazorpayOrderID   string     `json:"razorpayOrderId"`
	RazorpayPaymentID string     `json:"razorpayPaymentId"`
	CustomerName      string     `json:"customerName"`
	CustomerEmail     string     `json:"customerEmail"`
	CustomerPhone     string     `json:"customerPhone"`
	TotalAmount       float64    `json:"totalAmount"`
	OrderDate         time.Time  `json:"orderDate"`
	ItemsJSON         string     `json:"itemsJson"`
	OriginalOrderID   *int64     `json:"originalOrderId,omitempty"` // past orders only
	Status            string     `json:"status,omitempty"`          // past orders only
	CompletedDate     *time.Time `json:"completedDate,omitempty"`   // past orders only
}

const liveCols = "id, razorpay_order_id, razorpay_payment_id, customer_name, customer_email, customer_phone, total_amount, order_date, items_json"
const pastCols = liveCols + ", original_order_id, status, completed_date"

func (a *App) queryOrders(past bool, where string, args ...any) ([]Order, error) {
	table, cols := "live_orders", liveCols
	if past {
		table, cols = "past_orders", pastCols
	}
	rows, err := a.db.Query("SELECT "+cols+" FROM "+table+" WHERE "+where+" ORDER BY order_date DESC", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		var o Order
		dst := []any{&o.ID, &o.RazorpayOrderID, &o.RazorpayPaymentID, &o.CustomerName, &o.CustomerEmail,
			&o.CustomerPhone, &o.TotalAmount, &o.OrderDate, &o.ItemsJSON}
		if past {
			dst = append(dst, &o.OriginalOrderID, &o.Status, &o.CompletedDate)
		}
		if err := rows.Scan(dst...); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (a *App) liveOrders(w http.ResponseWriter, r *http.Request, cid int64) {
	orders, err := a.queryOrders(false, "canteen_id=?", cid)
	if err != nil {
		fail(w, 500, "db error")
		return
	}
	writeJSON(w, 200, orders)
}

func (a *App) pastOrders(w http.ResponseWriter, r *http.Request, cid int64) {
	orders, err := a.queryOrders(true, "canteen_id=?", cid)
	if err != nil {
		fail(w, 500, "db error")
		return
	}
	writeJSON(w, 200, orders)
}

// myOrders is unauthenticated (the phone number is the customer's only identity).
// shortcut: anyone who knows a phone number can read its orders, add an OTP if that matters.
func (a *App) myOrders(w http.ResponseWriter, r *http.Request) {
	phone := r.URL.Query().Get("phone")
	if phone == "" {
		fail(w, 400, "phone required")
		return
	}
	cid := a.canteenID(w, r)
	if cid == 0 {
		return
	}
	live, err1 := a.queryOrders(false, "canteen_id=? AND customer_phone=?", cid, phone)
	past, err2 := a.queryOrders(true, "canteen_id=? AND customer_phone=?", cid, phone)
	if err1 != nil || err2 != nil {
		fail(w, 500, "db error")
		return
	}
	writeJSON(w, 200, map[string]any{"liveOrders": live, "pastOrders": past})
}

// finish moves a live order to past_orders with the given status, inside one transaction.
func (a *App) finish(status string) func(http.ResponseWriter, *http.Request, int64) {
	return func(w http.ResponseWriter, r *http.Request, cid int64) {
		id := r.PathValue("id")
		tx, err := a.db.Begin()
		if err != nil {
			fail(w, 500, "db error")
			return
		}
		defer tx.Rollback()

		var lock int64 // row lock so a double click can't move the order twice
		if err := tx.QueryRow("SELECT id FROM live_orders WHERE id=? AND canteen_id=? FOR UPDATE", id, cid).Scan(&lock); err != nil {
			fail(w, 404, "live order not found")
			return
		}
		res, err := tx.Exec(`INSERT INTO past_orders
			(canteen_id, original_order_id, razorpay_order_id, razorpay_payment_id, customer_name, customer_email,
			 customer_phone, total_amount, order_date, items_json, status, completed_date)
			SELECT canteen_id, id, razorpay_order_id, razorpay_payment_id, customer_name, customer_email,
			 customer_phone, total_amount, order_date, items_json, ?, ?
			FROM live_orders WHERE id=?`, status, time.Now().UTC(), lock)
		if err != nil {
			fail(w, 500, "db error")
			return
		}
		if _, err := tx.Exec("DELETE FROM live_orders WHERE id=?", lock); err != nil {
			fail(w, 500, "db error")
			return
		}
		if err := tx.Commit(); err != nil {
			fail(w, 500, "db error")
			return
		}
		pid, _ := res.LastInsertId()
		past, err := a.queryOrders(true, "id=?", pid)
		if err != nil || len(past) != 1 {
			fail(w, 500, "db error")
			return
		}
		writeJSON(w, 200, past[0])
	}
}

// ---- Razorpay ----

func rupeesToPaise(rupees float64) int64 { return int64(math.Round(rupees * 100)) }

// validSignature checks Razorpay's checkout signature: HMAC-SHA256(order_id|payment_id) hex.
func validSignature(secret, orderID, paymentID, signature string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(orderID + "|" + paymentID))
	want := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(signature))
}

func (a *App) rzp(method, path string, body any) (int, []byte, error) {
	var rd io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "https://api.razorpay.com/v1"+path, rd)
	if err != nil {
		return 0, nil, err
	}
	req.SetBasicAuth(a.rzpKey, a.rzpSecret)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{Timeout: 15 * time.Second}).Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	return resp.StatusCode, data, err
}

// createPayment opens a Razorpay order; the frontend passes the response to the checkout popup.
func (a *App) createPayment(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Amount  float64 `json:"amount"`
		Receipt string  `json:"receipt"`
	}
	if !decode(w, r, &b) {
		return
	}
	paise := rupeesToPaise(b.Amount)
	if paise < 100 {
		fail(w, 400, "amount must be at least 1 rupee")
		return
	}
	// The amount is re-checked against DB prices in verifyPayment before any order is saved.
	status, data, err := a.rzp("POST", "/orders", map[string]any{"amount": paise, "currency": "INR", "receipt": b.Receipt})
	if err != nil || status >= 300 {
		log.Printf("razorpay create order failed: status=%d err=%v body=%s", status, err, data)
		fail(w, http.StatusBadGateway, "payment gateway error")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

type cartLine struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

func (a *App) verifyPayment(w http.ResponseWriter, r *http.Request) {
	var b struct {
		PaymentID    string `json:"razorpay_payment_id"`
		OrderID      string `json:"razorpay_order_id"`
		Signature    string `json:"razorpay_signature"`
		CustomerInfo struct {
			Name  string `json:"name"`
			Email string `json:"email"`
			Phone string `json:"phone"`
		} `json:"customerInfo"`
		CartItems []struct {
			ID       int64 `json:"id"`
			Quantity int   `json:"quantity"`
		} `json:"cartItems"`
	}
	if !decode(w, r, &b) {
		return
	}
	if !validSignature(a.rzpSecret, b.OrderID, b.PaymentID, b.Signature) {
		fail(w, 400, "Payment verification failed: Invalid signature")
		return
	}
	cid := a.canteenID(w, r)
	if cid == 0 {
		return
	}
	if b.CustomerInfo.Name == "" || b.CustomerInfo.Phone == "" || len(b.CartItems) == 0 {
		fail(w, 400, "customer name, phone and cart items required")
		return
	}

	// Name, price and total come from the DB, never from the browser.
	lines := make([]cartLine, 0, len(b.CartItems))
	var total float64
	for _, c := range b.CartItems {
		if c.Quantity <= 0 {
			fail(w, 400, "quantity must be positive")
			return
		}
		l := cartLine{ID: c.ID, Quantity: c.Quantity}
		if err := a.db.QueryRow("SELECT name, price FROM items WHERE id=? AND canteen_id=?", c.ID, cid).Scan(&l.Name, &l.Price); err != nil {
			fail(w, 400, "item not found in this canteen")
			return
		}
		total += l.Price * float64(l.Quantity)
		lines = append(lines, l)
	}

	// The amount Razorpay actually charged must equal the DB-computed total.
	status, data, err := a.rzp("GET", "/orders/"+b.OrderID, nil)
	var rz struct {
		Amount int64 `json:"amount"`
	}
	if err != nil || status >= 300 || json.Unmarshal(data, &rz) != nil {
		log.Printf("razorpay fetch order failed: status=%d err=%v", status, err)
		fail(w, http.StatusBadGateway, "payment gateway error")
		return
	}
	if rz.Amount != rupeesToPaise(total) {
		log.Printf("AMOUNT MISMATCH, REFUND NEEDED: payment=%s charged=%d expected=%d", b.PaymentID, rz.Amount, rupeesToPaise(total))
		fail(w, 400, "paid amount does not match the cart total")
		return
	}

	itemsJSON, _ := json.Marshal(lines)
	_, err = a.db.Exec(`INSERT INTO live_orders
		(canteen_id, razorpay_order_id, razorpay_payment_id, customer_name, customer_email, customer_phone, total_amount, order_date, items_json)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		cid, b.OrderID, b.PaymentID, b.CustomerInfo.Name, b.CustomerInfo.Email, b.CustomerInfo.Phone, total, time.Now().UTC(), string(itemsJSON))
	var me *mysql.MySQLError
	switch {
	case errors.As(err, &me) && me.Number == 1062: // same payment verified twice
		writeJSON(w, 200, map[string]string{"message": "order already saved"})
	case err != nil:
		log.Printf("CRITICAL: payment %s verified but order save failed: %v", b.PaymentID, err)
		fail(w, 500, fmt.Sprintf("Payment verified but order save failed. Quote payment id %s to support.", b.PaymentID))
	default:
		writeJSON(w, 200, map[string]string{"message": "Payment verified and order saved successfully"})
	}
}
