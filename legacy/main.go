package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
)

type App struct {
	db             *sql.DB
	jwtSecret      []byte
	rzpKey         string
	rzpSecret      string
	defaultCanteen string
}

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		log.Fatalf("missing required env var %s (see .env.example)", k)
	}
	return v
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// decode reads a JSON body (max 1MB) into v; writes 400 and returns false on bad input.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

// canteenID resolves the canteen a customer is ordering from: X-Canteen header
// (set by the frontend after a QR scan), then ?canteen=, then the default.
func (a *App) canteenID(w http.ResponseWriter, r *http.Request) int64 {
	slug := r.Header.Get("X-Canteen")
	if slug == "" {
		slug = r.URL.Query().Get("canteen")
	}
	if slug == "" {
		slug = a.defaultCanteen
	}
	var id int64
	if err := a.db.QueryRow("SELECT id FROM canteens WHERE slug=?", slug).Scan(&id); err != nil {
		fail(w, http.StatusNotFound, "canteen not found")
		return 0
	}
	return id
}

func cors(origin string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Canteen")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) getCanteen(w http.ResponseWriter, r *http.Request) {
	var id int64
	var name string
	slug := r.PathValue("slug")
	if err := a.db.QueryRow("SELECT id, name FROM canteens WHERE slug=?", slug).Scan(&id, &name); err != nil {
		fail(w, http.StatusNotFound, "canteen not found")
		return
	}
	writeJSON(w, 200, map[string]any{"id": id, "name": name, "slug": slug})
}

// seedAdmin creates the first admin for the default canteen from .env, so there is
// no open register endpoint. shortcut: one-off seed, add an admin-management endpoint if staff change often.
func (a *App) seedAdmin() {
	var n int
	if err := a.db.QueryRow("SELECT COUNT(*) FROM admins").Scan(&n); err != nil || n > 0 {
		return
	}
	user, pass := os.Getenv("ADMIN_USERNAME"), os.Getenv("ADMIN_PASSWORD")
	if user == "" || pass == "" {
		log.Println("no admin exists; set ADMIN_USERNAME and ADMIN_PASSWORD in .env and restart")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	_, err = a.db.Exec("INSERT INTO admins (username, password_hash, canteen_id) SELECT ?, ?, id FROM canteens WHERE slug=?",
		user, string(hash), a.defaultCanteen)
	if err != nil {
		log.Fatal("seed admin: ", err)
	}
	log.Println("created admin", user, "for canteen", a.defaultCanteen)
}

func main() {
	// .env is optional: real environment variables win, so deployments can skip the file.
	_ = godotenv.Load()

	cfg, err := mysql.ParseDSN(mustEnv("DB_DSN"))
	if err != nil {
		log.Fatal("bad DB_DSN: ", err)
	}
	cfg.ParseTime = true // order_date etc. scan into time.Time
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		log.Fatal(err)
	}
	db := sql.OpenDB(connector)
	if err := db.Ping(); err != nil {
		log.Fatal("cannot connect to database (check DB_DSN and that schema.sql was run): ", err)
	}

	a := &App{
		db:             db,
		jwtSecret:      []byte(mustEnv("JWT_SECRET")),
		rzpKey:         mustEnv("RAZORPAY_KEY_ID"),
		rzpSecret:      mustEnv("RAZORPAY_KEY_SECRET"),
		defaultCanteen: env("DEFAULT_CANTEEN", "main-canteen"),
	}
	if len(a.jwtSecret) < 32 {
		log.Fatal("JWT_SECRET must be at least 32 characters")
	}
	a.seedAdmin()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/canteens/{slug}", a.getCanteen)

	// menu (public read, admin write)
	mux.HandleFunc("GET /api/v1/items", a.listItems)
	mux.HandleFunc("GET /api/v1/category/{category}", a.listItems)
	mux.HandleFunc("POST /api/v1/items", a.admin(a.addItem))

	// payment + customer orders
	mux.HandleFunc("POST /api/create-order", a.createPayment)
	mux.HandleFunc("POST /api/verify-payment", a.verifyPayment)
	mux.HandleFunc("GET /api/admin/my-orders", a.myOrders)

	// admin
	mux.HandleFunc("POST /auth/login", a.login)
	mux.HandleFunc("GET /api/admin/live-orders", a.admin(a.liveOrders))
	mux.HandleFunc("GET /api/admin/past-orders", a.admin(a.pastOrders))
	mux.HandleFunc("POST /api/admin/complete/{id}", a.admin(a.finish("COMPLETED")))
	mux.HandleFunc("POST /api/admin/reject/{id}", a.admin(a.finish("REJECTED")))

	srv := &http.Server{
		Addr:              env("ADDR", ":8080"),
		Handler:           cors(env("FRONTEND_ORIGIN", "http://localhost:3000"), mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Println("listening on", srv.Addr)
	log.Fatal(srv.ListenAndServe())
}
