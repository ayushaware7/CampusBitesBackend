package menu

import (
	"net/http"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/authctx"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

// Register mounts the public menu and the admin management routes. requireAdmin is the auth
// middleware; it is injected so this package never imports the auth module.
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

	mux.HandleFunc("GET /api/v1/canteens/{slug}/menu", httpx.Handle(h.publicMenu))

	admin("GET /api/v1/admin/menu", h.adminMenu)

	admin("POST /api/v1/admin/categories", h.saveCategory)
	admin("PUT /api/v1/admin/categories/{id}", h.saveCategory)
	admin("DELETE /api/v1/admin/categories/{id}", h.deleteCategory)

	admin("POST /api/v1/admin/items", h.saveItem)
	admin("PUT /api/v1/admin/items/{id}", h.saveItem)
	admin("PATCH /api/v1/admin/items/{id}/availability", h.setAvailability)
	admin("DELETE /api/v1/admin/items/{id}", h.deleteItem)

	admin("GET /api/v1/admin/addon-groups", h.listGroups)
	admin("POST /api/v1/admin/addon-groups", h.saveGroup)
	admin("PUT /api/v1/admin/addon-groups/{id}", h.saveGroup)
	admin("DELETE /api/v1/admin/addon-groups/{id}", h.deleteGroup)
}

// optionalID is 0 for create routes (no {id} in the path) and the parsed id for update routes.
func optionalID(r *http.Request) (int64, error) {
	if r.PathValue("id") == "" {
		return 0, nil
	}
	return httpx.PathID(r, "id")
}

func (h *Handler) publicMenu(w http.ResponseWriter, r *http.Request) error {
	cats, err := h.svc.MenuBySlug(r.Context(), r.PathValue("slug"))
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"categories": cats})
	return nil
}

func (h *Handler) adminMenu(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	cats, err := h.svc.Menu(r.Context(), canteenID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"categories": cats})
	return nil
}

func (h *Handler) saveCategory(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := optionalID(r)
	if err != nil {
		return err
	}
	var in CategoryInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	var cat *Category
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
		cat, err = h.svc.CreateCategory(r.Context(), canteenID, in)
	} else {
		cat, err = h.svc.UpdateCategory(r.Context(), canteenID, id, in)
	}
	if err != nil {
		return err
	}
	httpx.JSON(w, status, cat)
	return nil
}

func (h *Handler) deleteCategory(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteCategory(r.Context(), canteenID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) saveItem(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := optionalID(r)
	if err != nil {
		return err
	}
	var in ItemInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	item, err := h.svc.SaveItem(r.Context(), canteenID, id, in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	httpx.JSON(w, status, item)
	return nil
}

func (h *Handler) setAvailability(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	var in struct {
		IsAvailable *bool `json:"isAvailable"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	if in.IsAvailable == nil {
		return apperr.Invalid("isAvailable is required")
	}
	if err := h.svc.SetItemAvailability(r.Context(), canteenID, id, *in.IsAvailable); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) deleteItem(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteItem(r.Context(), canteenID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) listGroups(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	groups, err := h.svc.AddonGroups(r.Context(), canteenID)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, groups)
	return nil
}

func (h *Handler) saveGroup(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := optionalID(r)
	if err != nil {
		return err
	}
	var in AddonGroupInput
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	g, err := h.svc.SaveAddonGroup(r.Context(), canteenID, id, in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	httpx.JSON(w, status, g)
	return nil
}

func (h *Handler) deleteGroup(w http.ResponseWriter, r *http.Request, canteenID int64) error {
	id, err := httpx.PathID(r, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteAddonGroup(r.Context(), canteenID, id); err != nil {
		return err
	}
	w.WriteHeader(http.StatusNoContent)
	return nil
}
