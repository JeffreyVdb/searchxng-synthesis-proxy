// Package main is the entry point for the search synthesis proxy server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/api"
	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/config"
	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/llm"
	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/proxy"
	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/searx"
)

// Build metadata injected via ldflags.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	srv, err := newServer(cfg, logger)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("starting server",
		slog.String("addr", cfg.Addr()),
		slog.String("version", version),
		slog.String("commit", commit),
	)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	logger.Info("server stopped")
	return nil
}

func newServer(cfg config.Config, logger *slog.Logger) (*http.Server, error) {
	searchHTTP := &http.Client{Timeout: cfg.SearchTimeout}
	llmHTTP := &http.Client{Timeout: cfg.LLMTimeout}

	searxClient := searx.NewClient(cfg.SearXNGBaseURL, searchHTTP)
	llmClient := llm.NewClient(llm.Options{
		APIKey:  cfg.LLMAPIKey,
		BaseURL: cfg.LLMBaseURL,
		Model:   cfg.LLMModel,
		Referer: cfg.OpenRouterReferer,
		Title:   cfg.OpenRouterTitle,
	}, llmHTTP)

	proxySvc := proxy.NewService(searxClient, llmClient, proxy.Options{
		MaxSearchResults: cfg.MaxSearchResults,
	})

	handler := api.NewHandler(logger, proxySvc)

	return &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}, nil
}
