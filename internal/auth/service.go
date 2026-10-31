// Package auth handles canteen-admin login and protects admin routes with a JWT.
package auth

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/authctx"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

const tokenTTL = time.Hour

// Compared against when the username is unknown, so login timing does not reveal valid usernames.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password"), bcrypt.DefaultCost)

type claims struct {
	CanteenID int64 `json:"cid"`
	jwt.RegisteredClaims
}

type Service struct {
	store  *Store
	secret []byte
}

func NewService(store *Store, secret []byte) *Service { return &Service{store, secret} }

func (s *Service) Login(ctx context.Context, username, password string) (string, error) {
	hash, canteenID, found, err := s.store.ByUsername(ctx, username)
	if err != nil {
		return "", err
	}
	if !found {
		hash = string(dummyHash)
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil || !found {
		return "", apperr.Unauthorized("invalid username or password")
	}
	now := time.Now()
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		CanteenID: canteenID,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}).SignedString(s.secret)
}

// Seed creates the first admin from config when the table is empty. There is deliberately no public register endpoint.
func (s *Service) Seed(ctx context.Context, username, password, canteenSlug string) error {
	n, err := s.store.Count(ctx)
	if err != nil || n > 0 {
		return err
	}
	if username == "" || password == "" {
		log.Println("no admin exists; set ADMIN_USERNAME and ADMIN_PASSWORD to create one")
		return nil
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.store.Create(ctx, username, string(hash), canteenSlug); err != nil {
		return err
	}
	log.Printf("created admin %q for canteen %q", username, canteenSlug)
	return nil
}

// Require rejects requests without a valid Bearer token and stores the admin's canteen in the context.
func (s *Service) Require(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			httpx.WriteError(w, apperr.Unauthorized("login required"))
			return
		}
		var c claims
		_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return s.secret, nil },
			jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			httpx.WriteError(w, apperr.Unauthorized("invalid or expired token"))
			return
		}
		next.ServeHTTP(w, r.WithContext(authctx.WithCanteen(r.Context(), c.CanteenID)))
	})
}
