package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/gui/internal/dashboard"
	"github.com/glebarez/sqlite"
	"github.com/pressly/goose/v3"
	"gorm.io/gorm"
)

func main() {
	addr := envOrDefault("GUI_ADDR", ":8090")
	dbPath := envOrDefault("GUI_DB", "gui.db")

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

	app := dashboard.New(&http.Client{Timeout: 10 * time.Second}, 15*time.Second, db)
	if err := app.BootstrapAdmin(
		envOrDefault("GUI_ADMIN_EMAIL", "admin@local"),
		envOrDefault("GUI_ADMIN_PASSWORD", "admin123"),
	); err != nil {
		log.Fatal(err)
	}
	go app.RunSyncLoop(context.Background())

	log.Printf("dashboard listening on %s", addr)
	if err := http.ListenAndServe(addr, app.Routes()); err != nil {
		log.Fatal(err)
	}
}

func envOrDefault(name, fallback string) string {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return fallback
	}
	return v
}
