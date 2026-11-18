package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kevindurb/todo-example/internal/todo"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "todos.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestOpenCreatesDatabaseAndSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "todos.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("database file was not created: %v", err)
	}
	var version int
	if err := st.db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read user_version: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("user_version = %d, want %d", version, schemaVersion)
	}
	if _, err := st.List(context.Background()); err != nil {
		t.Fatalf("List on fresh store: %v", err)
	}
}

func TestOpenRepeatedStartPreservesTodos(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.db")

	first, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}
	created, err := first.Create(context.Background(), todo.Input{Title: "persist me", Notes: "notes"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	second, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer second.Close()

	listed, err := second.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("got %d todos after restart, want 1", len(listed))
	}
	if listed[0].ID != created.ID || listed[0].Title != created.Title {
		t.Errorf("todo after restart = %+v, want id %d title %q", listed[0], created.ID, created.Title)
	}
}

func TestOpenUsesCustomPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom", "location", "custom.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer st.Close()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("custom database path was not used: %v", err)
	}
}

func TestOpenUnusablePathNamesPath(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blocker, []byte("file"), 0o644); err != nil {
		t.Fatalf("write blocker: %v", err)
	}
	path := filepath.Join(blocker, "todos.db")

	_, err := Open(path)
	if err == nil {
		t.Fatal("Open succeeded, want an error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("error %q does not name the path %q", err, path)
	}
}

func TestOpenRejectsNewerSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "todos.db")
	st, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if _, err := st.db.Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatalf("set newer version: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if _, err := Open(path); err == nil {
		t.Fatal("Open succeeded on a newer schema, want an error")
	}
}

func TestCRUD(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	created, err := st.Create(ctx, todo.Input{Title: "first", Notes: "some notes"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == 0 || created.Done || created.Title != "first" {
		t.Fatalf("Create returned %+v", created)
	}

	got, err := st.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "first" || got.Notes != "some notes" {
		t.Errorf("Get = %+v", got)
	}

	time.Sleep(2 * time.Millisecond)
	updated, err := st.Update(ctx, created.ID, todo.Input{Title: "second", Notes: ""})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Title != "second" || updated.Notes != "" {
		t.Errorf("Update = %+v", updated)
	}
	if !updated.CreatedAt.Equal(created.CreatedAt) {
		t.Errorf("CreatedAt changed from %v to %v", created.CreatedAt, updated.CreatedAt)
	}
	if !updated.UpdatedAt.After(created.UpdatedAt) {
		t.Errorf("UpdatedAt did not advance: %v then %v", created.UpdatedAt, updated.UpdatedAt)
	}

	done, err := st.SetDone(ctx, created.ID, true)
	if err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	if !done.Done {
		t.Error("SetDone(true) left the todo open")
	}
	again, err := st.SetDone(ctx, created.ID, true)
	if err != nil {
		t.Fatalf("SetDone again: %v", err)
	}
	if !again.Done || again.ID != created.ID {
		t.Errorf("repeated SetDone = %+v", again)
	}
	reopened, err := st.SetDone(ctx, created.ID, false)
	if err != nil {
		t.Fatalf("SetDone(false): %v", err)
	}
	if reopened.Done {
		t.Error("SetDone(false) left the todo done")
	}

	if err := st.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := st.Get(ctx, created.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get after delete error = %v, want ErrNotFound", err)
	}
}

func TestUpdatePreservesCompletionState(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	created, err := st.Create(ctx, todo.Input{Title: "one"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := st.SetDone(ctx, created.ID, true); err != nil {
		t.Fatalf("SetDone: %v", err)
	}
	updated, err := st.Update(ctx, created.ID, todo.Input{Title: "renamed"})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !updated.Done {
		t.Error("Update cleared the completion state")
	}
}

func TestNotFound(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	if _, err := st.Get(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get error = %v, want ErrNotFound", err)
	}
	if _, err := st.Update(ctx, 404, todo.Input{Title: "x"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("Update error = %v, want ErrNotFound", err)
	}
	if _, err := st.SetDone(ctx, 404, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("SetDone error = %v, want ErrNotFound", err)
	}
	if err := st.Delete(ctx, 404); !errors.Is(err, ErrNotFound) {
		t.Errorf("Delete error = %v, want ErrNotFound", err)
	}
}

func TestListOrdersOpenBeforeDoneAndNewestFirst(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	first, err := st.Create(ctx, todo.Input{Title: "first"})
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}
	second, err := st.Create(ctx, todo.Input{Title: "second"})
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}
	third, err := st.Create(ctx, todo.Input{Title: "third"})
	if err != nil {
		t.Fatalf("Create third: %v", err)
	}
	if _, err := st.SetDone(ctx, first.ID, true); err != nil {
		t.Fatalf("SetDone: %v", err)
	}

	assertOrder(t, st, "third", "second", "first")

	if _, err := st.SetDone(ctx, third.ID, true); err != nil {
		t.Fatalf("SetDone third: %v", err)
	}
	assertOrder(t, st, "second", "third", "first")

	if second.ID == 0 {
		t.Fatal("second todo was not created")
	}
}

func assertOrder(t *testing.T, st *Store, want ...string) {
	t.Helper()
	listed, err := st.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != len(want) {
		t.Fatalf("List returned %d todos, want %d", len(listed), len(want))
	}
	for i, title := range want {
		if listed[i].Title != title {
			t.Errorf("position %d = %q, want %q", i, listed[i].Title, title)
		}
	}
}

func TestSQLMetacharactersAreStoredLiterally(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	if _, err := st.Create(ctx, todo.Input{Title: "keep me"}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	hostile := `"'; DROP TABLE todos; --`
	created, err := st.Create(ctx, todo.Input{Title: hostile, Notes: `notes with 'quotes' and "double"`})
	if err != nil {
		t.Fatalf("Create hostile title: %v", err)
	}
	got, err := st.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != hostile {
		t.Errorf("title = %q, want %q", got.Title, hostile)
	}
	listed, err := st.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != 2 {
		t.Errorf("List returned %d todos, want 2", len(listed))
	}
}

func TestConcurrentCreatesAllPersist(t *testing.T) {
	st := openTestStore(t)
	ctx := context.Background()

	const count = 25
	var wg sync.WaitGroup
	errs := make(chan error, count)
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := st.Create(ctx, todo.Input{Title: fmt.Sprintf("todo-%02d", i)}); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Create: %v", err)
	}

	listed, err := st.List(ctx)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(listed) != count {
		t.Errorf("List returned %d todos, want %d", len(listed), count)
	}
}
