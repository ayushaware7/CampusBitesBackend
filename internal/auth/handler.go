package auth

import (
	"net/http"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Handler struct{ svc *Service }

func NewHandler(svc *Service) *Handler { return &Handler{svc} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /auth/login", httpx.Handle(h.login))
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var in struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := httpx.Decode(w, r, &in); err != nil {
		return err
	}
	token, err := h.svc.Login(r.Context(), in.Username, in.Password)
	if err != nil {
		return err
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"token": token})
	return nil
}
