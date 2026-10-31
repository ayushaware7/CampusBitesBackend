// Package httpx has the small HTTP helpers every module handler shares.
package httpx

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

func JSON(c *gin.Context, status int, v any) { c.JSON(status, v) }

var statusByKind = map[apperr.Kind]int{
	apperr.KindInvalid:      http.StatusBadRequest,
	apperr.KindNotFound:     http.StatusNotFound,
	apperr.KindUnauthorized: http.StatusUnauthorized,
	apperr.KindConflict:     http.StatusConflict,
	apperr.KindUnavailable:  http.StatusBadGateway,
}

// WriteError sends app errors with their message; anything else is logged and hidden behind a 500.
func WriteError(c *gin.Context, err error) {
	if kind, ok := apperr.KindOf(err); ok {
		c.JSON(statusByKind[kind], gin.H{"error": err.Error()})
		return
	}
	log.Printf("internal error: %v", err)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
}

// HandlerFunc is a gin handler that returns its error instead of writing it.
type HandlerFunc func(c *gin.Context) error

func Handle(fn HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := fn(c); err != nil {
			WriteError(c, err)
		}
	}
}

// Decode reads a JSON body (max 1MB) and rejects unknown fields so typos fail loudly.
func Decode(c *gin.Context, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return apperr.Invalid("invalid request body: %v", err)
	}
	return nil
}

func PathID(c *gin.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, apperr.Invalid("invalid %s", name)
	}
	return id, nil
}

func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.Writer.Header()
		h.Set("Access-Control-Allow-Origin", origin)
		h.Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
