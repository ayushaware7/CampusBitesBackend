// Package canteen looks canteens up by the slug in their QR code.
package canteen

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Canteen struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db} }

func (s *Store) BySlug(ctx context.Context, slug string) (*Canteen, error) {
	c := Canteen{Slug: slug}
	err := s.db.QueryRowContext(ctx, `SELECT id, name FROM canteens WHERE slug=? AND is_active=1`, slug).Scan(&c.ID, &c.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("canteen not found")
	}
	return &c, err
}

// IDBySlug lets other modules resolve a QR slug without importing this package's types.
func (s *Store) IDBySlug(ctx context.Context, slug string) (int64, error) {
	c, err := s.BySlug(ctx, slug)
	if err != nil {
		return 0, err
	}
	return c.ID, nil
}

type Handler struct{ store *Store }

func NewHandler(store *Store) *Handler { return &Handler{store} }

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/canteens/{slug}", httpx.Handle(func(w http.ResponseWriter, r *http.Request) error {
		c, err := h.store.BySlug(r.Context(), r.PathValue("slug"))
		if err != nil {
			return err
		}
		httpx.JSON(w, http.StatusOK, c)
		return nil
	}))
}
