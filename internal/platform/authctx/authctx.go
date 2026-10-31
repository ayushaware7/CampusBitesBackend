// Package authctx carries the logged-in admin's canteen through the request context,
// so feature modules can read it without importing the auth module.
package authctx

import (
	"context"

	"github.com/ayushaware7/CampusBitesBackend/internal/platform/apperr"
)

type key struct{}

func WithCanteen(ctx context.Context, canteenID int64) context.Context {
	return context.WithValue(ctx, key{}, canteenID)
}

// CanteenID returns the admin's canteen, or an Unauthorized error if the route was not behind auth.
func CanteenID(ctx context.Context) (int64, error) {
	id, ok := ctx.Value(key{}).(int64)
	if !ok {
		return 0, apperr.Unauthorized("login required")
	}
	return id, nil
}
