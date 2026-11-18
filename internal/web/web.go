// Package web serves the server-rendered todo interface.
package web

import (
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/kevindurb/todo-example/internal/store"
	"github.com/kevindurb/todo-example/internal/todo"
)

const (
	maxBodyBytes  = 64 << 10
	healthTimeout = 2 * time.Second
)

//go:embed templates/*.html static/*
var assets embed.FS

// Server renders the todo pages and owns the HTTP routes.
type Server struct {
	store     *store.Store
	logger    *slog.Logger
	templates *template.Template
	staticFS  fs.FS
}

// New parses the embedded templates and returns a server for the given store.
func New(st *store.Store, logger *slog.Logger) (*Server, error) {
	if st == nil {
		return nil, errors.New("web: store is required")
	}
	if logger == nil {
		logger = slog.Default()
	}
	templates, err := template.ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("web: parse templates: %w", err)
	}
	staticFS, err := fs.Sub(assets, "static")
	if err != nil {
		return nil, fmt.Errorf("web: embedded static assets: %w", err)
	}
	return &Server{store: st, logger: logger, templates: templates, staticFS: staticFS}, nil
}

// Handler returns the routed handler with logging and security headers.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleList)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.HandleFunc("GET /todos/new", s.handleNewForm)
	mux.HandleFunc("POST /todos", s.handleCreate)
	mux.HandleFunc("GET /todos/{id}/edit", s.handleEditForm)
	mux.HandleFunc("POST /todos/{id}/edit", s.handleUpdate)
	mux.HandleFunc("POST /todos/{id}/status", s.handleSetStatus)
	mux.HandleFunc("GET /todos/{id}/delete", s.handleDeleteConfirm)
	mux.HandleFunc("POST /todos/{id}/delete", s.handleDelete)
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(s.staticFS)))

	return s.withLogging(s.withSecurityHeaders(mux))
}

type pageData struct {
	Title       string
	Heading     string
	Message     string
	Todos       []todo.Todo
	Form        todo.Input
	Errors      map[string]string
	Action      string
	SubmitLabel string
	Todo        *todo.Todo
}

func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	todos, err := s.store.List(r.Context())
	if err != nil {
		s.serverError(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "list.html", pageData{Title: "Todos", Todos: todos})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthTimeout)
	defer cancel()

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	if err := s.store.Ping(ctx); err != nil {
		s.logger.Error("health check failed", "error", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("unavailable\n"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok\n"))
}

func (s *Server) handleNewForm(w http.ResponseWriter, r *http.Request) {
	s.render(w, http.StatusOK, "form.html", pageData{
		Title:       "Add a todo",
		Heading:     "Add a todo",
		Action:      "/todos",
		SubmitLabel: "Add todo",
	})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	in, ok := s.parseForm(w, r)
	if !ok {
		return
	}
	validated, err := todo.Validate(in)
	if err != nil {
		s.renderValidationError(w, r, "/todos", "Add a todo", "Add todo", in, err)
		return
	}
	if _, err := s.store.Create(r.Context(), validated); err != nil {
		s.serverError(w, r, err)
		return
	}
	s.redirectToList(w, r)
}

func (s *Server) handleEditForm(w http.ResponseWriter, r *http.Request) {
	id, ok := s.todoID(w, r)
	if !ok {
		return
	}
	found, err := s.store.Get(r.Context(), id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "form.html", pageData{
		Title:       "Edit todo",
		Heading:     "Edit todo",
		Action:      fmt.Sprintf("/todos/%d/edit", id),
		SubmitLabel: "Save changes",
		Form:        todo.Input{Title: found.Title, Notes: found.Notes},
	})
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := s.todoID(w, r)
	if !ok {
		return
	}
	in, ok := s.parseForm(w, r)
	if !ok {
		return
	}
	validated, err := todo.Validate(in)
	if err != nil {
		s.renderValidationError(w, r, fmt.Sprintf("/todos/%d/edit", id), "Edit todo", "Save changes", in, err)
		return
	}
	if _, err := s.store.Update(r.Context(), id, validated); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.redirectToList(w, r)
}

func (s *Server) handleSetStatus(w http.ResponseWriter, r *http.Request) {
	id, ok := s.todoID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := r.ParseForm(); err != nil {
		s.logger.Warn("reject status request", "path", r.URL.Path, "error", err)
		s.renderError(w, http.StatusBadRequest, "Bad request", "The requested completion state could not be read.")
		return
	}

	var done bool
	switch r.PostFormValue("done") {
	case "true":
		done = true
	case "false":
		done = false
	default:
		s.renderError(w, http.StatusBadRequest, "Bad request", "The requested completion state is not valid.")
		return
	}
	if _, err := s.store.SetDone(r.Context(), id, done); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.redirectToList(w, r)
}

func (s *Server) handleDeleteConfirm(w http.ResponseWriter, r *http.Request) {
	id, ok := s.todoID(w, r)
	if !ok {
		return
	}
	found, err := s.store.Get(r.Context(), id)
	if err != nil {
		s.storeError(w, r, err)
		return
	}
	s.render(w, http.StatusOK, "confirm.html", pageData{Title: "Delete todo", Todo: &found})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := s.todoID(w, r)
	if !ok {
		return
	}
	if err := s.store.Delete(r.Context(), id); err != nil {
		s.storeError(w, r, err)
		return
	}
	s.redirectToList(w, r)
}

func (s *Server) parseForm(w http.ResponseWriter, r *http.Request) (todo.Input, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := r.ParseForm(); err != nil {
		s.logger.Warn("reject form submission", "path", r.URL.Path, "error", err)
		s.renderError(w, http.StatusBadRequest, "Bad request",
			"The form submission could not be read. It may be malformed or too large.")
		return todo.Input{}, false
	}
	return todo.Input{
		Title: r.PostFormValue("title"),
		Notes: r.PostFormValue("notes"),
	}, true
}

func (s *Server) renderValidationError(w http.ResponseWriter, r *http.Request, action, heading, submit string, in todo.Input, err error) {
	var invalid *todo.ValidationError
	if !errors.As(err, &invalid) {
		s.serverError(w, r, err)
		return
	}
	s.render(w, http.StatusBadRequest, "form.html", pageData{
		Title:       heading,
		Heading:     heading,
		Action:      action,
		SubmitLabel: submit,
		Form:        in,
		Errors:      invalid.Fields,
	})
}

func (s *Server) todoID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil || id < 1 {
		s.renderError(w, http.StatusNotFound, "Todo not found", "That todo does not exist.")
		return 0, false
	}
	return id, true
}

func (s *Server) storeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, store.ErrNotFound) {
		s.renderError(w, http.StatusNotFound, "Todo not found", "That todo does not exist.")
		return
	}
	s.serverError(w, r, err)
}

func (s *Server) serverError(w http.ResponseWriter, r *http.Request, err error) {
	s.logger.Error("request failed", "method", r.Method, "path", r.URL.Path, "error", err)
	s.renderError(w, http.StatusInternalServerError, "Something went wrong",
		"The request could not be completed. Try again.")
}

func (s *Server) redirectToList(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) renderError(w http.ResponseWriter, status int, heading, message string) {
	s.render(w, status, "error.html", pageData{Title: heading, Heading: heading, Message: message})
}

func (s *Server) render(w http.ResponseWriter, status int, name string, data pageData) {
	var buf bytes.Buffer
	if err := s.templates.ExecuteTemplate(&buf, name, data); err != nil {
		s.logger.Error("render template", "template", name, "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if _, err := buf.WriteTo(w); err != nil {
		s.logger.Warn("write response", "template", name, "error", err)
	}
}

func (s *Server) withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; style-src 'self'; form-action 'self'; frame-ancestors 'none'; base-uri 'none'")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Referrer-Policy", "same-origin")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(p)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		s.logger.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", status,
			"duration", time.Since(start))
	})
}
