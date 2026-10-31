// Command server wires the modules together and serves the HTTP API.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/ayushaware7/CampusBitesBackend/internal/auth"
	"github.com/ayushaware7/CampusBitesBackend/internal/canteen"
	"github.com/ayushaware7/CampusBitesBackend/internal/config"
	"github.com/ayushaware7/CampusBitesBackend/internal/menu"
	"github.com/ayushaware7/CampusBitesBackend/internal/order"
	"github.com/ayushaware7/CampusBitesBackend/internal/payment"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/database"
	"github.com/ayushaware7/CampusBitesBackend/internal/platform/httpx"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	db, err := database.Open(cfg.DBDSN, false)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	canteens := canteen.NewStore(db)
	authSvc := auth.NewService(auth.NewStore(db), []byte(cfg.JWTSecret))
	menuSvc := menu.NewService(menu.NewMySQLRepository(db), canteens)

	// order and payment need each other (order opens a payment, payment reports "paid"), so one
	// side is bound late through a closure. They still only know each other by interface.
	var orderSvc *order.Service
	paymentSvc := payment.NewService(
		payment.NewMySQLRepository(db),
		payment.NewRazorpay(cfg.RazorpayKeyID, cfg.RazorpayKeySecret),
		payment.NotifierFunc(func(ctx context.Context, orderID int64) error { return orderSvc.MarkPaid(ctx, orderID) }),
		cfg.RazorpayKeySecret, cfg.RazorpayWebhookSecret,
	)
	orderSvc = order.NewService(order.NewMySQLRepository(db), canteens, menuPricer{menuSvc}, paymentSvc)

	if err := authSvc.Seed(context.Background(), cfg.AdminUsername, cfg.AdminPassword, cfg.DefaultCanteen); err != nil {
		log.Printf("warning: could not seed admin (run migrations and create the canteen first): %v", err)
	}

	router := gin.New()
	router.Use(gin.Recovery(), httpx.CORS(cfg.FrontendOrigin))
	canteen.NewHandler(canteens).Register(router, authSvc.Require)
	auth.NewHandler(authSvc).Register(router)
	menu.NewHandler(menuSvc).Register(router, authSvc.Require)
	order.NewHandler(orderSvc).Register(router, authSvc.Require)
	payment.NewHandler(paymentSvc).Register(router)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go func() {
		log.Println("listening on", cfg.Addr)
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}
