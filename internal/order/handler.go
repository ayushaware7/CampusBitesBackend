package order

import (
	"net/http"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/authctx"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

// Register mounts the customer routes and the admin routes (behind requireAdmin).
func (h *Handler) Register(mux *http.ServeMux, requireAdmin func(http.Handler) http.Handler) {
	admin := func(pattern string, fn func(w http.ResponseWriter, r *http.Request, canteenID int64) error) {
		mux.Handle(pattern, requireAdmin(httpx.Handle(func(w http.ResponseWriter, r *http.Request) error {
			canteenID, err := authctx.CanteenID(r.Context())
			if err != nil {
				return err
			}
			return fn(w, r, canteenID)
		})))
	}

	mux.HandleFunc("POST /api/v1/canteens/{slug}/orders", httpx.Handle(h.place))
	mux.HandleFunc("GET /api/v1/canteens/{slug}/orders/track", httpx.Handle(h.track))

	admin("GET /api/v1/admin/orders", h.list)
	admin("PATCH /api/v1/admin/orders/{id}/status", h.setStatus)
}

func (h *Handler) place(w http.ResponseWriter, r *http.Request) error {
	var in PlaceInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	placed, err := h.svc.Place(r.Context(), r.PathValue("slug"), in)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusCreated, placed)
	return nil
}

func (h *Handler) track(w http.ResponseWriter, r *http.Request) error {
	q := r.URL.Query()
	o, err := h.svc.Track(r.Context(), r.PathValue("slug"), q.Get("code"), q.Get("phone"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, o)
	return nil
}

func (h *Handler) list(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	var orders []Order
	var err error
	switch view := r.URL.Query().Get("view"); view {
	case "live":
		orders, err = h.svc.Live(r.Context(), canteenID)
	case "past":
		orders, err = h.svc.Past(r.Context(), canteenID)
	default:
		return apperr.Invalid("view must be live or past")
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, orders)
	return nil
}

func (h *Handler) setStatus(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		Status Status `json:"status"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	o, err := h.svc.SetStatus(r.Context(), canteenID, id, in.Status)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, o)
	return nil
}
