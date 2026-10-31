package payment

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Razorpay implements Gateway against the Razorpay REST API.
type Razorpay struct {
	keyID, keySecret string
	client           *http.Client
}

func NewRazorpay(keyID, keySecret string) *Razorpay {
	return &Razorpay{keyID, keySecret, &http.Client{Timeout: 15 * time.Second}}
}

func (r *Razorpay) CreateOrder(ctx context.Context, amountPaise int64, receipt string) (string, error) {
	body, _ := json.Marshal(map[string]any{"amount": amountPaise, "currency": "INR", "receipt": receipt})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(r.keyID, r.keySecret)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("razorpay returned %d: %s", resp.StatusCode, data)
	}
	var out struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.ID == "" {
		return "", fmt.Errorf("razorpay: unexpected response: %s", data)
	}
	return out.ID, nil
}
