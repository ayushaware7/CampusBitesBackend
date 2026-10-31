package menu

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

// Register mounts the public menu and the admin management routes. requireAdmin is the auth
// middleware; it is injected so this package never imports the auth module.
func (h *Handler) Register(r *gin.Engine, requireAdmin gin.HandlerFunc) {
	r.GET("/api/v1/canteens/:slug/menu", httpx.Handle(h.publicMenu))

	admin := r.Group("/api/v1/admin", requireAdmin)
	admin.GET("/menu", withCanteen(h.adminMenu))

	admin.POST("/categories", withCanteen(h.saveCategory))
	admin.PUT("/categories/:id", withCanteen(h.saveCategory))
	admin.DELETE("/categories/:id", withCanteen(h.deleteCategory))

	admin.POST("/items", withCanteen(h.saveItem))
	admin.PUT("/items/:id", withCanteen(h.saveItem))
	admin.PATCH("/items/:id/availability", withCanteen(h.setAvailability))
	admin.DELETE("/items/:id", withCanteen(h.deleteItem))

	admin.GET("/addon-groups", withCanteen(h.listGroups))
	admin.POST("/addon-groups", withCanteen(h.saveGroup))
	admin.PUT("/addon-groups/:id", withCanteen(h.saveGroup))
	admin.DELETE("/addon-groups/:id", withCanteen(h.deleteGroup))
}

// optionalID is 0 for create routes (no :id in the path) and the parsed id for update routes.
func optionalID(c *gin.Context) (int64, error) {
	if c.Param("id") == "" {
		return 0, nil
	}
	return httpx.PathID(c, "id")
}

func (h *Handler) publicMenu(c *gin.Context) error {
	cats, err := h.svc.MenuBySlug(c.Request.Context(), c.Param("slug"))
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, gin.H{"categories": cats})
	return nil
}

func (h *Handler) adminMenu(c *gin.Context, canteenID int64) error {
	cats, err := h.svc.Menu(c.Request.Context(), canteenID)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, gin.H{"categories": cats})
	return nil
}

func (h *Handler) saveCategory(c *gin.Context, canteenID int64) error {
	id, err := optionalID(c)
	if err != nil {
		return err
	}
	var in CategoryInput
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	var cat *Category
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
		cat, err = h.svc.CreateCategory(c.Request.Context(), canteenID, in)
	} else {
		cat, err = h.svc.UpdateCategory(c.Request.Context(), canteenID, id, in)
	}
	if err != nil {
		return err
	}
	httpx.JSON(c, status, cat)
	return nil
}

func (h *Handler) deleteCategory(c *gin.Context, canteenID int64) error {
	id, err := httpx.PathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteCategory(c.Request.Context(), canteenID, id); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}

func (h *Handler) saveItem(c *gin.Context, canteenID int64) error {
	id, err := optionalID(c)
	if err != nil {
		return err
	}
	var in ItemInput
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	item, err := h.svc.SaveItem(c.Request.Context(), canteenID, id, in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	httpx.JSON(c, status, item)
	return nil
}

func (h *Handler) setAvailability(c *gin.Context, canteenID int64) error {
	id, err := httpx.PathID(c, "id")
	if err != nil {
		return err
	}
	var in struct {
		IsAvailable *bool `json:"isAvailable"`
	}
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	if in.IsAvailable == nil {
		return apperr.Invalid("isAvailable is required")
	}
	if err := h.svc.SetItemAvailability(c.Request.Context(), canteenID, id, *in.IsAvailable); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}

func (h *Handler) deleteItem(c *gin.Context, canteenID int64) error {
	id, err := httpx.PathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteItem(c.Request.Context(), canteenID, id); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}

func (h *Handler) listGroups(c *gin.Context, canteenID int64) error {
	groups, err := h.svc.AddonGroups(c.Request.Context(), canteenID)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, groups)
	return nil
}

func (h *Handler) saveGroup(c *gin.Context, canteenID int64) error {
	id, err := optionalID(c)
	if err != nil {
		return err
	}
	var in AddonGroupInput
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	g, err := h.svc.SaveAddonGroup(c.Request.Context(), canteenID, id, in)
	if err != nil {
		return err
	}
	status := http.StatusOK
	if id == 0 {
		status = http.StatusCreated
	}
	httpx.JSON(c, status, g)
	return nil
}

func (h *Handler) deleteGroup(c *gin.Context, canteenID int64) error {
	id, err := httpx.PathID(c, "id")
	if err != nil {
		return err
	}
	if err := h.svc.DeleteAddonGroup(c.Request.Context(), canteenID, id); err != nil {
		return err
	}
	c.Status(http.StatusNoContent)
	return nil
}
