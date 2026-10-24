// Package httpx has the small HTTP helpers every module handler shares.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("write response: %v", err)
	}
}

var statusByKind = map[apperr.Kind]int{
	apperr.KindInvalid:      http.StatusBadRequest,
	apperr.KindNotFound:     http.StatusNotFound,
	apperr.KindUnauthorized: http.StatusUnauthorized,
	apperr.KindConflict:     http.StatusConflict,
	apperr.KindUnavailable:  http.StatusBadGateway,
}

// WriteError sends app errors with their message; anything else is logged and hidden behind a 500.
func WriteError(w http.ResponseWriter, err error) {
	if kind, ok := apperr.KindOf(err); ok {
		JSON(w, statusByKind[kind], map[string]string{"error": err.Error()})
		return
	}
	log.Printf("internal error: %v", err)
	JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
}

// HandlerFunc is an http handler that returns its error instead of writing it.
type HandlerFunc func(w http.ResponseWriter, r *http.Request) error

func Handle(fn HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := fn(w, r); err != nil {
			WriteError(w, err)
		}
	}
}

// Decode reads a JSON body (max 1MB) and rejects unknown fields so typos fail loudly.
func Decode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperr.Invalid("invalid request body: %v", err)
	}
	return nil
}

func PathID(r *http.Request, name string) (int64, error) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Invalid("invalid %s", name)
	}
	return id, nil
}

func CORS(origin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
