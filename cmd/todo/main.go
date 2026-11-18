// Command todo serves the server-rendered todo application.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kevindurb/todo-example/internal/store"
	"github.com/kevindurb/todo-example/internal/web"
)

const (
	defaultAddr         = ":8080"
	defaultDatabasePath = "./data/todos.db"
	shutdownTimeout     = 10 * time.Second
)

type config struct {
	Addr         string
	DatabasePath string
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

func loadConfig() (config, error) {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = defaultAddr
	}
	if err := validateAddr(addr); err != nil {
		return config{}, err
	}

	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = defaultDatabasePath
	}
	return config{Addr: addr, DatabasePath: path}, nil
}

func validateAddr(addr string) error {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid ADDR %q: %w", addr, err)
	}
	if port == "" {
		return fmt.Errorf("invalid ADDR %q: missing port", addr)
	}
	return nil
}

func run(logger *slog.Logger) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	st, err := store.Open(cfg.DatabasePath)
	if err != nil {
		return err
	}

	app, err := web.New(st, logger)
	if err != nil {
		st.Close()
		return err
	}

	listener, err := net.Listen("tcp", cfg.Addr)
	if err != nil {
		st.Close()
		return fmt.Errorf("listen on ADDR %q: %w", cfg.Addr, err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 2)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signals)

	go func() {
		first := <-signals
		logger.Info("shutdown requested", "signal", first.String())
		cancel()
		second := <-signals
		logger.Warn("second shutdown signal, exiting immediately", "signal", second.String())
		os.Exit(1)
	}()

	logger.Info("listening", "addr", cfg.Addr, "database", cfg.DatabasePath)
	if err := serve(ctx, listener, app.Handler(), logger); err != nil {
		st.Close()
		return err
	}
	return st.Close()
}

// serve runs the HTTP server until ctx is cancelled, then drains in-flight
// requests for at most shutdownTimeout.
func serve(ctx context.Context, listener net.Listener, handler http.Handler, logger *slog.Logger) error {
	server := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
	}

	logger.Info("draining connections")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	return nil
}
