// Package main is the entry point for the MCP SSE server binary.
//
// The MCP server is a separate process from the main search synthesis proxy.
// It exposes the search capability as an MCP tool over SSE, calling the main
// proxy's /v1/search API as its upstream.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/config"
	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/mcpserver"
	"github.com/JeffreyVdb/searchxng-synthesis-proxy/internal/synthproxy"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.LoadMCP()
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

	logger.Info("starting MCP server",
		slog.String("addr", cfg.Addr()),
		slog.String("upstream", cfg.ProxyBaseURL),
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

func newServer(cfg config.MCPConfig, logger *slog.Logger) (*http.Server, error) {
	httpClient := synthproxy.DefaultHTTPClient(cfg.RequestTimeout)
	client := synthproxy.NewClient(cfg.ProxyBaseURL, httpClient)

	handler := mcpserver.NewSSEHandler(logger, client, mcpserver.SSEOptions{
		SSEEndpoint:     "/mcp/sse",
		MessageEndpoint: "/mcp/messages",
	})

	return &http.Server{
		Addr:         cfg.Addr(),
		Handler:      handler,
		ReadTimeout:  cfg.ReadTimeout,
		// WriteTimeout is intentionally 0 (or a long value) to keep SSE
		// connections alive. Do NOT set a short write timeout here — it
		// will break long-lived event streams.
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}, nil
}
