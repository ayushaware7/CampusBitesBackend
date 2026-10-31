// Package config reads settings from the environment (and an optional .env file).
package config

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	DBDSN          string
	JWTSecret      string
	FrontendOrigin string
	Addr           string
	DefaultCanteen string // canteen the first admin is attached to
	AdminUsername  string // optional: first admin, created only if none exists
	AdminPassword  string

	RazorpayKeyID         string
	RazorpayKeySecret     string
	RazorpayWebhookSecret string // optional: the webhook endpoint stays disabled without it
}

// LoadEnv loads .env if present; real environment variables always win.
func LoadEnv() { _ = godotenv.Load() }

// DSN returns DB_DSN for tools (migrate) that need nothing else.
func DSN() (string, error) {
	LoadEnv()
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		return "", fmt.Errorf("missing required env var DB_DSN (see .env.example)")
	}
	return dsn, nil
}

func Load() (Config, error) {
	LoadEnv()
	dsn, err := DSN()
	if err != nil {
		return Config{}, err
	}
	c := Config{
		DBDSN:          dsn,
		JWTSecret:      os.Getenv("JWT_SECRET"),
		FrontendOrigin: get("FRONTEND_ORIGIN", "http://localhost:3000"),
		Addr:           get("ADDR", ":8080"),
		DefaultCanteen: get("DEFAULT_CANTEEN", "main-canteen"),
		AdminUsername:  os.Getenv("ADMIN_USERNAME"),
		AdminPassword:  os.Getenv("ADMIN_PASSWORD"),

		RazorpayKeyID:         os.Getenv("RAZORPAY_KEY_ID"),
		RazorpayKeySecret:     os.Getenv("RAZORPAY_KEY_SECRET"),
		RazorpayWebhookSecret: os.Getenv("RAZORPAY_WEBHOOK_SECRET"),
	}
	if len(c.JWTSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must be set and at least 32 characters")
	}
	if c.RazorpayKeyID == "" || c.RazorpayKeySecret == "" {
		return Config{}, fmt.Errorf("RAZORPAY_KEY_ID and RAZORPAY_KEY_SECRET are required (see .env.example)")
	}
	return c, nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
