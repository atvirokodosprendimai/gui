package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/atvirokodosprendimai/gui/internal/dashboard"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func main() {
	addr := envOrDefault("GUI_ADDR", ":8090")
	dbPath := envOrDefault("GUI_DB", "gui.db")
	trustProxy := strings.EqualFold(strings.TrimSpace(os.Getenv("GUI_TRUST_PROXY")), "true")

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(sqlDB, "migrations"); err != nil {
		log.Fatal(err)
	}

	app := dashboard.New(&http.Client{Timeout: 10 * time.Second}, 15*time.Second, trustProxy, db)
	if err := app.InitError(); err != nil {
		log.Fatal(err)
	}
	adminEmail := strings.TrimSpace(os.Getenv("GUI_ADMIN_EMAIL"))
	adminPassword := strings.TrimSpace(os.Getenv("GUI_ADMIN_PASSWORD"))
	if adminEmail != "" && adminPassword != "" {
		if err := app.BootstrapAdmin(adminEmail, adminPassword); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("admin bootstrap skipped: set GUI_ADMIN_EMAIL and GUI_ADMIN_PASSWORD")
	}
	if natsURL := strings.TrimSpace(os.Getenv("GUI_NATS_URL")); natsURL != "" {
		if err := app.ConfigureNATS(natsURL); err != nil {
			log.Fatal(err)
		}
		log.Printf("nats notifier enabled")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	syncDone := make(chan struct{})
	go func() {
		defer close(syncDone)
		app.RunSyncLoop(ctx)
	}()

	srv := &http.Server{
		Addr:              addr,
		Handler:           app.Routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	log.Printf("dashboard listening on %s", addr)
	errCh := make(chan error, 1)
	go func() {
		errCh <- srv.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = app.Close(shutdownCtx)
		_ = srv.Shutdown(shutdownCtx)
		select {
		case <-syncDone:
		case <-shutdownCtx.Done():
		}
		_ = sqlDB.Close()
	case err := <-errCh:
		if err != nil && err != http.ErrServerClosed {
			_ = app.Close(context.Background())
			_ = sqlDB.Close()
			log.Fatal(err)
		}
	}
}

func envOrDefault(name, fallback string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	return v
}
