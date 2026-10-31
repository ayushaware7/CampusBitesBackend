package payment

import (
	"io"
	"net/http"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/payments/verify", httpx.Handle(h.verify))
	mux.HandleFunc("POST /api/v1/payments/webhook", httpx.Handle(h.webhook))
}

func (h *Handler) verify(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		OrderID           int64  `json:"orderId"`
		RazorpayPaymentID string `json:"razorpayPaymentId"`
		RazorpaySignature string `json:"razorpaySignature"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if err := h.svc.Confirm(r.Context(), in.OrderID, in.RazorpayPaymentID, in.RazorpaySignature); err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "paid"})
	return nil
}

// webhook reads the raw body: the signature is computed over the exact bytes Razorpay sent.
func (h *Handler) webhook(w http.ResponseWriter, r *http.Request) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := h.svc.HandleWebhook(r.Context(), body, r.Header.Get("X-Razorpay-Signature")); err != nil {
		return err
	}
	w.WriteHeader(http.StatusOK)
	return nil
}
