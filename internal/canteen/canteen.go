// Package canteen looks canteens up by the slug in their QR code and lets an admin edit their branding.
package canteen

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/authctx"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

type Canteen struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
	// Branding is optional; empty means the app uses its default look.
	Tagline      string `json:"tagline"`
	LogoURL      string `json:"logoUrl"`
	PrimaryColor string `json:"primaryColor"` // "#rrggbb"
}

// BrandingInput is what an admin can change about their own canteen.
type BrandingInput struct {
	Name         string `json:"name"`
	Tagline      string `json:"tagline"`
	LogoURL      string `json:"logoUrl"`
	PrimaryColor string `json:"primaryColor"`
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// validate trims the input in place and checks every field; optional fields may be empty.
func (in *BrandingInput) validate() error {
	in.Name, in.Tagline = strings.TrimSpace(in.Name), strings.TrimSpace(in.Tagline)
	in.LogoURL, in.PrimaryColor = strings.TrimSpace(in.LogoURL), strings.TrimSpace(in.PrimaryColor)
	if in.Name == "" || utf8.RuneCountInString(in.Name) > 100 {
		return apperr.Invalid("name is required and must be at most 100 characters")
	}
	if utf8.RuneCountInString(in.Tagline) > 200 {
		return apperr.Invalid("tagline must be at most 200 characters")
	}
	if in.PrimaryColor != "" && !hexColor.MatchString(in.PrimaryColor) {
		return apperr.Invalid("primaryColor must look like #0066CC")
	}
	if in.LogoURL != "" {
		u, err := url.Parse(in.LogoURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || len(in.LogoURL) > 500 {
			return apperr.Invalid("logoUrl must be an http(s) link of at most 500 characters")
		}
	}
	return nil
}

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db} }

const cols = `id, name, slug, tagline, logo_url, primary_color`

func (s *Store) one(ctx context.Context, where string, arg any) (*Canteen, error) {
	var c Canteen
	var tagline, logo, color sql.NullString
	err := s.db.QueryRowContext(ctx, `SELECT `+cols+` FROM canteens WHERE `+where+` AND is_active=1`, arg).
		Scan(&c.ID, &c.Name, &c.Slug, &tagline, &logo, &color)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, apperr.NotFound("canteen not found")
	}
	c.Tagline, c.LogoURL, c.PrimaryColor = tagline.String, logo.String, color.String
	return &c, err
}

func (s *Store) BySlug(ctx context.Context, slug string) (*Canteen, error) {
	return s.one(ctx, `slug=?`, slug)
}

func (s *Store) ByID(ctx context.Context, id int64) (*Canteen, error) {
	return s.one(ctx, `id=?`, id)
}

// UpdateBranding replaces the branding of one canteen; empty optional fields are stored as NULL.
func (s *Store) UpdateBranding(ctx context.Context, id int64, in BrandingInput) (*Canteen, error) {
	if err := in.validate(); err != nil {
		return nil, err
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE canteens SET name=?, tagline=NULLIF(?,''), logo_url=NULLIF(?,''), primary_color=NULLIF(?,'')
		 WHERE id=? AND is_active=1`, in.Name, in.Tagline, in.LogoURL, in.PrimaryColor, id)
	if err != nil {
		return nil, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return nil, apperr.NotFound("canteen not found")
	}
	return s.ByID(ctx, id)
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

// Register mounts the public lookup and the admin branding routes (behind requireAdmin).
func (h *Handler) Register(r *gin.Engine, requireAdmin gin.HandlerFunc) {
	r.GET("/api/v1/canteens/:slug", httpx.Handle(func(c *gin.Context) error {
		cn, err := h.store.BySlug(c.Request.Context(), c.Param("slug"))
		if err != nil {
			return err
		}
		httpx.JSON(c, http.StatusOK, cn)
		return nil
	}))

	admin := r.Group("/api/v1/admin", requireAdmin)
	admin.GET("/canteen", httpx.Handle(h.mine))
	admin.PUT("/canteen", httpx.Handle(h.updateMine))
}

func (h *Handler) mine(c *gin.Context) error {
	id, err := authctx.CanteenID(c.Request.Context())
	if err != nil {
		return err
	}
	cn, err := h.store.ByID(c.Request.Context(), id)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, cn)
	return nil
}

func (h *Handler) updateMine(c *gin.Context) error {
	id, err := authctx.CanteenID(c.Request.Context())
	if err != nil {
		return err
	}
	var in BrandingInput
	if err := httpx.Decode(c, &in); err != nil {
		return err
	}
	cn, err := h.store.UpdateBranding(c.Request.Context(), id, in)
	if err != nil {
		return err
	}
	httpx.JSON(c, http.StatusOK, cn)
	return nil
}
