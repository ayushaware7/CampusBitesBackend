package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(r *gin.Engine) {
	r.POST("/auth/login", httpx.Handle(h.login))
}

func (h *Handler) login(c *gin.Context) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	token, err := h.svc.Login(c.Request.Context(), in.Username, in.Password)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, gin.H{"token": token})
	return nil
}
