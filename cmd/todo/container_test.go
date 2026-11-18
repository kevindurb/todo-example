//go:build container

package main

import (
	"archive/tar"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestContainerImageAndPersistence builds the image, checks the runtime
// contents, and verifies that a todo created in one container is listed after a
// restart against the same volume.
//
// It is opt-in because it needs a container runtime and network access to pull
// the base images:
//
//	TODO_CONTAINER_TEST=1 go test -tags container ./cmd/todo
//
// Override the runtime with TODO_CONTAINER, for example
// "podman --root /path --runroot /path".
func TestContainerImageAndPersistence(t *testing.T) {
	if os.Getenv("TODO_CONTAINER_TEST") == "" {
		t.Skip("set TODO_CONTAINER_TEST=1 to run the container acceptance test")
	}
	runtime := strings.Fields(os.Getenv("TODO_CONTAINER"))
	if len(runtime) == 0 {
		runtime = []string{"docker"}
	}
	if _, err := exec.LookPath(runtime[0]); err != nil {
		t.Skipf("container runtime %q is not available", runtime[0])
	}

	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repository root: %v", err)
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	image := "todo-example:container-test"
	volume := "todo-example-test-" + suffix
	first := "todo-example-first-" + suffix
	second := "todo-example-second-" + suffix
	inspect := "todo-example-inspect-" + suffix

	run := func(args ...string) string {
		t.Helper()
		full := append(append([]string{}, runtime[1:]...), args...)
		cmd := exec.Command(runtime[0], full...)
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %s: %v\n%s", runtime[0], strings.Join(full, " "), err, out)
		}
		return string(out)
	}
	cleanup := func(args ...string) {
		full := append(append([]string{}, runtime[1:]...), args...)
		cmd := exec.Command(runtime[0], full...)
		cmd.Dir = repoRoot
		_ = cmd.Run()
	}

	t.Cleanup(func() {
		for _, name := range []string{first, second, inspect} {
			cleanup("rm", "-f", name)
		}
		cleanup("volume", "rm", "-f", volume)
	})

	t.Log("building the image")
	run("build", "--tag", image, ".")

	if user := strings.TrimSpace(run("inspect", "--format", "{{.Config.User}}", image)); user != "65532:65532" {
		t.Errorf("runtime user = %q, want 65532:65532", user)
	}

	assertRuntimeContents(t, runtime, repoRoot, image, inspect)

	run("volume", "create", volume)
	port := freePort(t)
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	start := func(name string) {
		run("run", "--detach", "--name", name,
			"--publish", fmt.Sprintf("127.0.0.1:%d:8080", port),
			"--volume", volume+":/data",
			image)
		waitForHealth(t, baseURL+"/healthz")
	}

	start(first)
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	response, err := client.PostForm(baseURL+"/todos", url.Values{
		"title": {"persisted todo"},
		"notes": {"created inside the container"},
	})
	if err != nil {
		t.Fatalf("create todo: %v", err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusSeeOther {
		t.Fatalf("create status = %d, want %d", response.StatusCode, http.StatusSeeOther)
	}
	assertListed(t, baseURL, "persisted todo")

	run("stop", first)
	run("rm", "-f", first)

	start(second)
	assertListed(t, baseURL, "persisted todo")
}

func assertRuntimeContents(t *testing.T, runtime []string, repoRoot, image, container string) {
	t.Helper()
	full := append(append([]string{}, runtime[1:]...), "create", "--name", container, image)
	create := exec.Command(runtime[0], full...)
	create.Dir = repoRoot
	if out, err := create.CombinedOutput(); err != nil {
		t.Fatalf("create container for inspection: %v\n%s", err, out)
	}

	full = append(append([]string{}, runtime[1:]...), "export", container)
	export := exec.Command(runtime[0], full...)
	export.Dir = repoRoot
	stdout, err := export.StdoutPipe()
	if err != nil {
		t.Fatalf("export container: %v", err)
	}
	if err := export.Start(); err != nil {
		t.Fatalf("start export: %v", err)
	}

	files := make(map[string]bool)
	reader := tar.NewReader(stdout)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read exported image: %v", err)
		}
		name := strings.TrimSuffix(strings.TrimPrefix(header.Name, "./"), "/")
		files[name] = true
		if strings.HasSuffix(name, ".go") || strings.Contains(name, "usr/local/go") {
			t.Errorf("runtime image contains build material: %s", name)
		}
	}
	if err := export.Wait(); err != nil {
		t.Fatalf("export image: %v", err)
	}
	if !files["app/todo"] {
		t.Error("runtime image does not contain /app/todo")
	}
	if !files["data"] {
		t.Error("runtime image does not contain the /data directory")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find free port: %v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForHealth(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		response, err := http.Get(url)
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("%s did not become healthy", url)
}

func assertListed(t *testing.T, baseURL, title string) {
	t.Helper()
	response, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("list todos: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read list: %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("list status = %d, want %d", response.StatusCode, http.StatusOK)
	}
	if !strings.Contains(string(body), title) {
		t.Errorf("list page does not contain %q", title)
	}
}
