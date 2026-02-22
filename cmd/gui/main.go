package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/atvirokodosprendimai/gui/internal/dashboard"
)

func main() {
	addr := envOrDefault("GUI_ADDR", ":8090")

	app := dashboard.New(&http.Client{Timeout: 10 * time.Second}, 15*time.Second)
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
