package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"rill/internal/config"
	"rill/internal/server"
)

func main() {
	healthCheck := flag.Bool("health", false, "check the local /healthz endpoint and exit")
	flag.Parse()
	if *healthCheck {
		os.Exit(runHealthCheck())
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("configuration error", "err", err)
		os.Exit(1)
	}
	app, err := server.NewWithOptions(context.Background(), cfg, server.Options{Logger: logger})
	if err != nil {
		logger.Error("startup failed", "err", err)
		os.Exit(1)
	}
	defer app.Close()

	httpServer := &http.Server{
		Addr:              cfg.Listen,
		Handler:           app.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	shutdownSignal := make(chan os.Signal, 1)
	signal.Notify(shutdownSignal, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-shutdownSignal
		logger.Info("shutting down", "signal", sig.String())
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			logger.Warn("graceful shutdown failed", "err", err)
		}
	}()
	logger.Info("Rill listening", "addr", cfg.Listen, "data_dir", cfg.DataDir, "cache_dir", cfg.CacheDir)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("http server stopped", "err", err)
		app.Close()
		os.Exit(1)
	}
}

// runHealthCheck keeps the container healthcheck independent of curl/wget,
// which are not guaranteed in minimal runtime images.
func runHealthCheck() int {
	target := strings.TrimSpace(os.Getenv("RILL_HEALTH_URL"))
	if target == "" {
		target = "http://127.0.0.1:8080/healthz"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	response, err := client.Get(target)
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: unexpected status", response.StatusCode)
		return 1
	}
	return 0
}
