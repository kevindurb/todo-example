package web

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/kevindurb/todo-example/internal/store"
	"github.com/kevindurb/todo-example/internal/todo"
)

func newTestServer(t *testing.T) (*Server, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "todos.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	srv, err := New(st, slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}
	return srv, st
}

func request(srv *Server, method, target string, form url.Values) *httptest.ResponseRecorder {
	var body io.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	}
	req := httptest.NewRequest(method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func mustCreate(t *testing.T, st *store.Store, in todo.Input) todo.Todo {
	t.Helper()
	created, err := st.Create(context.Background(), in)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	return created
}

func TestListEmpty(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := request(srv, http.MethodGet, "/", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	body := rec.Body.String()
	for _, want := range []string{"No todos yet", `action="/todos"`, `for="title"`, `for="notes"`} {
		if !strings.Contains(body, want) {
			t.Errorf("empty list does not contain %q", want)
		}
	}
}

func TestListShowsTodosAndControls(t *testing.T) {
	srv, st := newTestServer(t)
	open := mustCreate(t, st, todo.Input{Title: "open todo", Notes: "notes here"})
	done := mustCreate(t, st, todo.Input{Title: "done todo"})
	if _, err := st.SetDone(context.Background(), done.ID, true); err != nil {
		t.Fatalf("SetDone: %v", err)
	}

	rec := request(srv, http.MethodGet, "/", nil)
	body := rec.Body.String()
	for _, want := range []string{
		open.Title,
		done.Title,
		"notes here",
		"Open",
		"Done",
		"/todos/" + itoa(open.ID) + "/edit",
		"/todos/" + itoa(done.ID) + "/status",
		"/todos/" + itoa(open.ID) + "/delete",
		"Mark done",
		"Reopen",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("list page does not contain %q", want)
		}
	}
}

func TestCreateRedirectsAndPersists(t *testing.T) {
	srv, st := newTestServer(t)
	rec := request(srv, http.MethodPost, "/todos", url.Values{
		"title": {"Buy milk"},
		"notes": {"two litres"},
	})

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if location := rec.Header().Get("Location"); location != "/" {
		t.Errorf("Location = %q, want /", location)
	}
	listed, err := st.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 || listed[0].Title != "Buy milk" || listed[0].Notes != "two litres" {
		t.Fatalf("stored todos = %+v", listed)
	}

	list := request(srv, http.MethodGet, "/", nil)
	if !strings.Contains(list.Body.String(), "Buy milk") {
		t.Error("list does not show the created todo")
	}
}

func TestCreateValidationFailureRedisplayForm(t *testing.T) {
	srv, st := newTestServer(t)
	rec := request(srv, http.MethodPost, "/todos", url.Values{
		"title": {"   "},
		"notes": {"keep these notes"},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	body := rec.Body.String()
	for _, want := range []string{"Title is required.", "keep these notes", `action="/todos"`} {
		if !strings.Contains(body, want) {
			t.Errorf("validation page does not contain %q", want)
		}
	}
	listed, err := st.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 0 {
		t.Errorf("rejected submission created %d todos", len(listed))
	}
}

func TestCreateRejectsOversizedTitle(t *testing.T) {
	srv, st := newTestServer(t)
	rec := request(srv, http.MethodPost, "/todos", url.Values{
		"title": {strings.Repeat("a", todo.MaxTitleLength+1)},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Title must be 200 characters or fewer.") {
		t.Error("validation page is missing the title message")
	}
	if listed, _ := st.List(context.Background()); len(listed) != 0 {
		t.Errorf("rejected submission created %d todos", len(listed))
	}
}

func TestCreateRejectsOversizedBody(t *testing.T) {
	srv, st := newTestServer(t)
	rec := request(srv, http.MethodPost, "/todos", url.Values{
		"title": {strings.Repeat("a", 100_000)},
	})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if listed, _ := st.List(context.Background()); len(listed) != 0 {
		t.Errorf("oversized submission created %d todos", len(listed))
	}
}

func TestEditFormAndUpdate(t *testing.T) {
	srv, st := newTestServer(t)
	created := mustCreate(t, st, todo.Input{Title: "old title", Notes: "old notes"})

	form := request(srv, http.MethodGet, "/todos/"+itoa(created.ID)+"/edit", nil)
	if form.Code != http.StatusOK {
		t.Fatalf("edit form status = %d, want %d", form.Code, http.StatusOK)
	}
	body := form.Body.String()
	for _, want := range []string{"old title", "old notes", `action="/todos/` + itoa(created.ID) + `/edit"`} {
		if !strings.Contains(body, want) {
			t.Errorf("edit form does not contain %q", want)
		}
	}

	rec := request(srv, http.MethodPost, "/todos/"+itoa(created.ID)+"/edit", url.Values{
		"title": {"new title"},
		"notes": {"new notes"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	updated, err := st.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if updated.Title != "new title" || updated.Notes != "new notes" {
		t.Errorf("updated todo = %+v", updated)
	}
	if updated.Done {
		t.Error("update changed the completion state")
	}
}

func TestEditValidationKeepsOriginal(t *testing.T) {
	srv, st := newTestServer(t)
	created := mustCreate(t, st, todo.Input{Title: "keep"})

	rec := request(srv, http.MethodPost, "/todos/"+itoa(created.ID)+"/edit", url.Values{
		"title": {"changed"},
		"notes": {strings.Repeat("b", todo.MaxNotesLength+1)},
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !strings.Contains(rec.Body.String(), "Notes must be 2000 characters or fewer.") {
		t.Error("validation page is missing the notes message")
	}
	stored, err := st.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Title != "keep" {
		t.Errorf("rejected edit changed the title to %q", stored.Title)
	}
}

func TestSetStatus(t *testing.T) {
	srv, st := newTestServer(t)
	created := mustCreate(t, st, todo.Input{Title: "toggle me"})

	rec := request(srv, http.MethodPost, "/todos/"+itoa(created.ID)+"/status", url.Values{"done": {"true"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("mark done status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	stored, err := st.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if !stored.Done {
		t.Error("todo is still open after marking done")
	}

	rec = request(srv, http.MethodPost, "/todos/"+itoa(created.ID)+"/status", url.Values{"done": {"false"}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("reopen status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	stored, err = st.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if stored.Done {
		t.Error("todo is still done after reopening")
	}

	rec = request(srv, http.MethodPost, "/todos/"+itoa(created.ID)+"/status", url.Values{"done": {"maybe"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid state status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	srv, st := newTestServer(t)
	created := mustCreate(t, st, todo.Input{Title: "delete me", Notes: "details"})

	confirm := request(srv, http.MethodGet, "/todos/"+itoa(created.ID)+"/delete", nil)
	if confirm.Code != http.StatusOK {
		t.Fatalf("confirmation status = %d, want %d", confirm.Code, http.StatusOK)
	}
	body := confirm.Body.String()
	for _, want := range []string{"delete me", `action="/todos/` + itoa(created.ID) + `/delete"`, "Cancel"} {
		if !strings.Contains(body, want) {
			t.Errorf("confirmation page does not contain %q", want)
		}
	}
	if _, err := st.Get(context.Background(), created.ID); err != nil {
		t.Fatalf("loading the confirmation page removed the todo: %v", err)
	}

	rec := request(srv, http.MethodPost, "/todos/"+itoa(created.ID)+"/delete", nil)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete status = %d, want %d", rec.Code, http.StatusSeeOther)
	}
	if _, err := st.Get(context.Background(), created.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("Get after delete error = %v, want ErrNotFound", err)
	}
}

func TestUnknownTodoIsNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	cases := []struct {
		method string
		target string
		form   url.Values
	}{
		{http.MethodGet, "/todos/999/edit", nil},
		{http.MethodPost, "/todos/999/edit", url.Values{"title": {"x"}}},
		{http.MethodPost, "/todos/999/edit", url.Values{"title": {""}}},
		{http.MethodPost, "/todos/999/status", url.Values{"done": {"true"}}},
		{http.MethodPost, "/todos/999/status", url.Values{"done": {"maybe"}}},
		{http.MethodGet, "/todos/999/delete", nil},
		{http.MethodPost, "/todos/999/delete", nil},
		{http.MethodGet, "/todos/not-a-number/edit", nil},
	}
	for _, tc := range cases {
		rec := request(srv, tc.method, tc.target, tc.form)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%s %s status = %d, want %d", tc.method, tc.target, rec.Code, http.StatusNotFound)
		}
	}
}

func TestUnknownRouteIsNotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	if rec := request(srv, http.MethodGet, "/nope", nil); rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := request(srv, http.MethodPut, "/todos/new", nil)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Errorf("Allow = %q, want it to name GET", allow)
	}
}

func TestSecurityHeaders(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := request(srv, http.MethodGet, "/", nil)

	for header, want := range map[string]string{
		"Content-Security-Policy": "default-src 'none'",
		"X-Content-Type-Options":  "nosniff",
		"X-Frame-Options":         "DENY",
		"Referrer-Policy":         "same-origin",
	} {
		if got := rec.Header().Get(header); !strings.Contains(got, want) {
			t.Errorf("%s = %q, want it to contain %q", header, got, want)
		}
	}
}

func TestUserContentIsEscaped(t *testing.T) {
	srv, st := newTestServer(t)
	mustCreate(t, st, todo.Input{
		Title: `<script>alert("x")</script>`,
		Notes: `"><img src="https://example.test/pixel" onerror="alert(1)">`,
	})

	body := request(srv, http.MethodGet, "/", nil).Body.String()
	if strings.Contains(body, "<script") {
		t.Error("rendered page contains a script element")
	}
	if strings.Contains(body, "<img") {
		t.Error("rendered page contains an injected image element")
	}
	if !strings.Contains(body, "&lt;script&gt;") {
		t.Error("rendered page does not show the escaped title")
	}
	if !strings.Contains(body, "&lt;img") {
		t.Error("rendered page does not show the escaped notes")
	}
	if strings.Contains(body, `onerror="alert(1)"`) {
		t.Error("rendered page contains an unescaped event handler")
	}
}

func TestErrorPageRenders(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := request(srv, http.MethodGet, "/todos/999/edit", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	body := rec.Body.String()
	for _, want := range []string{"Todo not found", "Back to the list"} {
		if !strings.Contains(body, want) {
			t.Errorf("error page does not contain %q", want)
		}
	}
	if strings.Contains(body, "<script") {
		t.Error("error page contains a script element")
	}
}

func TestPagesUseOnlyLocalAssets(t *testing.T) {
	srv, st := newTestServer(t)
	created := mustCreate(t, st, todo.Input{Title: "a todo"})

	pages := []string{
		"/",
		"/todos/new",
		"/todos/" + itoa(created.ID) + "/edit",
		"/todos/" + itoa(created.ID) + "/delete",
	}
	for _, page := range pages {
		rec := request(srv, http.MethodGet, page, nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("GET %s status = %d, want %d", page, rec.Code, http.StatusOK)
		}
		body := rec.Body.String()
		if strings.Contains(body, "<script") {
			t.Errorf("%s contains a script element", page)
		}
		for _, remote := range []string{"http://", "https://", "//cdn"} {
			if strings.Contains(body, remote) {
				t.Errorf("%s references a third-party asset (%s)", page, remote)
			}
		}
	}
}

func TestStylesheetIsServed(t *testing.T) {
	srv, _ := newTestServer(t)
	rec := request(srv, http.MethodGet, "/static/app.css", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", contentType)
	}
}

func TestHealthz(t *testing.T) {
	srv, st := newTestServer(t)

	healthy := request(srv, http.MethodGet, "/healthz", nil)
	if healthy.Code != http.StatusOK {
		t.Fatalf("healthy status = %d, want %d", healthy.Code, http.StatusOK)
	}

	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	unavailable := request(srv, http.MethodGet, "/healthz", nil)
	if unavailable.Code != http.StatusServiceUnavailable {
		t.Errorf("unavailable status = %d, want %d", unavailable.Code, http.StatusServiceUnavailable)
	}
}

func TestRequestLoggingOmitsFormValues(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "todos.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	var logs bytes.Buffer
	srv, err := New(st, slog.New(slog.NewJSONHandler(&logs, nil)))
	if err != nil {
		t.Fatalf("web.New: %v", err)
	}

	rec := request(srv, http.MethodPost, "/todos", url.Values{
		"title": {"secret title value"},
		"notes": {"secret notes value"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusSeeOther)
	}

	line := logs.String()
	for _, want := range []string{`"method":"POST"`, `"path":"/todos"`, `"status":303`, `"duration"`} {
		if !strings.Contains(line, want) {
			t.Errorf("log line %q does not contain %q", line, want)
		}
	}
	for _, leak := range []string{"secret title value", "secret notes value"} {
		if strings.Contains(line, leak) {
			t.Errorf("log line contains form value %q", leak)
		}
	}
}

func itoa(id int64) string {
	return strconv.FormatInt(id, 10)
}
