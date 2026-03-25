package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	httpapi "github.com/chentianyu/celestia/internal/api/http"
	runtimepkg "github.com/chentianyu/celestia/internal/core/runtime"
	sqlitestore "github.com/chentianyu/celestia/internal/storage/sqlite"
)

func main() {
	ctx := context.Background()
	dbPath := getenv("CELESTIA_DB_PATH", "./data/celestia.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatal(err)
	}

	store, err := sqlitestore.New(dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		_ = store.Close()
	}()
	if err := store.EnsureSchema(ctx); err != nil {
		log.Fatal(err)
	}

	runtime := runtimepkg.New(store)
	if err := runtime.Reconcile(ctx); err != nil {
		log.Printf("runtime reconcile error: %v", err)
	}

	server := httpapi.New(getenv("CELESTIA_ADDR", ":8080"), runtime)
	go func() {
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, context.Canceled) && !errors.Is(err, syscall.EINVAL) {
			log.Fatalf("server failed: %v", err)
		}
	}()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	<-signals

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := runtime.Shutdown(shutdownCtx); err != nil {
		log.Printf("runtime shutdown error: %v", err)
	}
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
