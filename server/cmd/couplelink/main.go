package main

import (
	"context"
	"couplelink/server/internal/api"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	// Standalone Windows/Linux releases must work without a Go installation or
	// an operating-system timezone database.
	_ "time/tzdata"
)

var version = "1.0-r9"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	database := os.Getenv("DATABASE_URL")
	if database == "" {
		log.Fatal("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(database)
	if err != nil {
		log.Fatal("invalid database configuration")
	}
	config.MaxConns = 8
	db, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		log.Fatal("database initialization failed")
	}
	defer db.Close()
	boot, stop := context.WithTimeout(ctx, 30*time.Second)
	defer stop()
	if err = api.Migrate(boot, db); err != nil {
		log.Fatal("database migration failed")
	}
	handler, err := api.New(db, api.Config{Version: version,
		RealtimeURL: os.Getenv("COUPLELINK_REALTIME_URL"), RealtimeAPIKey: os.Getenv("COUPLELINK_REALTIME_API_KEY"), RealtimeSecret: os.Getenv("COUPLELINK_REALTIME_SECRET"),
		AllowedOrigins: strings.Fields(os.Getenv("COUPLELINK_ALLOWED_ORIGINS"))})
	if err != nil {
		log.Fatal("API initialization failed")
	}
	go handler.RunRealtimeOutbox(ctx)
	go handler.RunAnniversaryReminders(ctx)
	addr := os.Getenv("COUPLELINK_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}
	server := &http.Server{Addr: addr, Handler: handler.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 * 1024}
	stopped := make(chan struct{})
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 10*time.Second)
		defer done()
		server.Shutdown(shutdown)
		close(stopped)
	}()
	log.Printf("CoupleLink %s listening at %s", version, addr)
	if err = server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal("HTTP server failed")
	}
	cancel()
	<-stopped
}
