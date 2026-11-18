package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kevindurb/todo-example/internal/store"
	"github.com/kevindurb/todo-example/internal/web"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("ADDR", "")
	t.Setenv("DATABASE_PATH", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Addr != defaultAddr {
		t.Errorf("Addr = %q, want %q", cfg.Addr, defaultAddr)
	}
	if cfg.DatabasePath != defaultDatabasePath {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, defaultDatabasePath)
	}
}

func TestLoadConfigOverrides(t *testing.T) {
	t.Setenv("ADDR", "127.0.0.1:9090")
	t.Setenv("DATABASE_PATH", "/tmp/custom.db")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.Addr != "127.0.0.1:9090" {
		t.Errorf("Addr = %q", cfg.Addr)
	}
	if cfg.DatabasePath != "/tmp/custom.db" {
		t.Errorf("DatabasePath = %q", cfg.DatabasePath)
	}
}

func TestLoadConfigRejectsInvalidAddr(t *testing.T) {
	t.Setenv("ADDR", "not-an-address")
	t.Setenv("DATABASE_PATH", "")

	_, err := loadConfig()
	if err == nil {
		t.Fatal("loadConfig succeeded, want an error")
	}
	if !strings.Contains(err.Error(), "not-an-address") {
		t.Errorf("error %q does not name the value", err)
	}
}

func TestValidateAddr(t *testing.T) {
	for _, valid := range []string{":8080", "127.0.0.1:0", "localhost:9090"} {
		if err := validateAddr(valid); err != nil {
			t.Errorf("validateAddr(%q) = %v", valid, err)
		}
	}
	for _, invalid := range []string{"", "8080", "localhost", "localhost:"} {
		if err := validateAddr(invalid); err == nil {
			t.Errorf("validateAddr(%q) succeeded, want an error", invalid)
		}
	}
}

func TestServeDrainsInflightRequest(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		w.WriteHeader(http.StatusOK)
	})

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- serve(ctx, listener, handler, discardLogger()) }()

	responseCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := http.Get("http://" + listener.Addr().String() + "/")
		if err != nil {
			errCh <- err
			return
		}
		responseCh <- resp
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("handler was not reached")
	}
	cancel()
	close(release)

	select {
	case resp := <-responseCh:
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("in-flight response status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	case err := <-errCh:
		t.Fatalf("in-flight request failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("in-flight request did not finish")
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}

	if _, err := http.Get("http://" + listener.Addr().String() + "/"); err == nil {
		t.Error("server still accepts connections after shutdown")
	}
}

func TestServeServesTodoAppAgainstTemporaryDatabase(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "todos.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer st.Close()

	app, err := web.New(st, discardLogger())
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serveDone := make(chan error, 1)
	go func() { serveDone <- serve(ctx, listener, app.Handler(), discardLogger()) }()

	baseURL := "http://" + listener.Addr().String()
	waitForStatus(t, baseURL+"/healthz", http.StatusOK)

	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	if !strings.Contains(string(body), "No todos yet") {
		t.Error("list page does not render the empty state")
	}

	cancel()
	select {
	case err := <-serveDone:
		if err != nil {
			t.Fatalf("serve: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}
}

func waitForStatus(t *testing.T, url string, want int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == want {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s did not return status %d", url, want)
}
