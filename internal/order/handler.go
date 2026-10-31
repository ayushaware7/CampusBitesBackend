package order

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/authctx"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

// adminFunc is a handler for routes behind the auth middleware; it receives the admin's canteen id.
type adminFunc func(c *gin.Context, canteenID int64) error

func withCanteen(fn adminFunc) gin.HandlerFunc {
	return httpx.Handle(func(c *gin.Context) error {
		canteenID, err := authctx.CanteenID(c.Request.Context())
		if err != nil {
			return err
		}
		return fn(c, canteenID)
	})
}

// Register mounts the customer routes and the admin routes (behind requireAdmin).
func (h *Handler) Register(r *gin.Engine, requireAdmin gin.HandlerFunc) {
	r.POST("/api/v1/canteens/:slug/orders", httpx.Handle(h.place))
	r.GET("/api/v1/canteens/:slug/orders/track", httpx.Handle(h.track))

	admin := r.Group("/api/v1/admin", requireAdmin)
	admin.GET("/orders", withCanteen(h.list))
	admin.PATCH("/orders/:id/status", withCanteen(h.setStatus))
}

func (h *Handler) place(c *gin.Context) error {
	var in PlaceInput
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	placed, err := h.svc.Place(c.Request.Context(), c.Param("slug"), in)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusCreated, placed)
	return nil
}

func (h *Handler) track(c *gin.Context) error {
	o, err := h.svc.Track(c.Request.Context(), c.Param("slug"), c.Query("code"), c.Query("phone"))
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, o)
	return nil
}

func (h *Handler) list(c *gin.Context, canteenID int64) error {
	var orders []Order
	var err error
	ctx := c.Request.Context()
	switch c.Query("view") {
	case "live":
		orders, err = h.svc.Live(ctx, canteenID)
	case "past":
		orders, err = h.svc.Past(ctx, canteenID)
	default:
		return apperr.Invalid("view must be live or past")
	}
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, orders)
	return nil
}

func (h *Handler) setStatus(c *gin.Context, canteenID int64) error {
	id, err := httpx.PathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		Status Status `json:"status"`
	}
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	o, err := h.svc.SetStatus(c.Request.Context(), canteenID, id, in.Status)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, o)
	return nil
}
