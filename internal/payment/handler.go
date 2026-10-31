package payment

import (
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(r *gin.Engine) {
	r.POST("/api/v1/payments/verify", httpx.Handle(h.verify))
	r.POST("/api/v1/payments/webhook", httpx.Handle(h.webhook))
}

func (h *Handler) verify(c *gin.Context) error {
	var in struct {
		OrderID           int64  `json:"orderId"`
		RazorpayPaymentID string `json:"razorpayPaymentId"`
		RazorpaySignature string `json:"razorpaySignature"`
	}
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	if err := h.svc.Confirm(c.Request.Context(), in.OrderID, in.RazorpayPaymentID, in.RazorpaySignature); err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, gin.H{"status": "paid"})
	return nil
}

// webhook reads the raw body: the signature is computed over the exact bytes Razorpay sent.
func (h *Handler) webhook(c *gin.Context) error {
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	if err != nil {
		return err
	}
	if err := h.svc.HandleWebhook(c.Request.Context(), body, c.GetHeader("X-Razorpay-Signature")); err != nil {
		return err
	}
	c.Status(http.StatusOK)
	return nil
}
