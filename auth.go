package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type claims struct {
	CanteenID int64 `json:"cid"`
	jwt.RegisteredClaims
}

func (a *App) login(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !decode(w, r, &b) {
		return
	}
	var hash string
	var cid int64
	err := a.db.QueryRow("SELECT password_hash, canteen_id FROM admins WHERE username=?", b.Username).Scan(&hash, &cid)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(hash), []byte(b.Password)) != nil {
		fail(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	now := time.Now()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims{
		CanteenID: cid,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   b.Username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	}).SignedString(a.jwtSecret)
	if err != nil {
		fail(w, 500, "could not sign token")
		return
	}
	writeJSON(w, 200, map[string]string{"token": tok})
}

// admin guards a handler with a valid Bearer JWT and passes the admin's canteen id,
// so an admin can only ever see and change their own canteen's data.
func (a *App) admin(next func(w http.ResponseWriter, r *http.Request, cid int64)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok {
			fail(w, http.StatusUnauthorized, "login required")
			return
		}
		var c claims
		_, err := jwt.ParseWithClaims(raw, &c, func(*jwt.Token) (any, error) { return a.jwtSecret, nil },
			jwt.WithValidMethods([]string{"HS256"}))
		if err != nil {
			fail(w, http.StatusUnauthorized, "invalid or expired token")
			return
		}
		next(w, r, c.CanteenID)
	}
}
