//go:build e2e

// Package e2e holds the end-to-end test of the service.
//
// It starts the real binary as a process, talks to it over real HTTP and checks what a
// client would see. Being behind a build tag, the ordinary `go test ./...` does not pay
// for it; `make e2e` runs it. The test builds the binary itself, so nothing has to be
// prepared in advance.
package e2e

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// binary is the service built once for the whole package.
var binary string

func TestMain(m *testing.M) {
	root, err := filepath.Abs("..")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: resolve the module root: %v\n", err)
		os.Exit(1)
	}

	dir, err := os.MkdirTemp("", "edh-stats-e2e-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "e2e: create a temporary directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(dir)

	binary = filepath.Join(dir, "edh-stats")
	build := exec.Command("go", "build", "-o", binary, "./cmd/edh-stats")
	build.Dir = root
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "e2e: build the service: %v\n", err)
		os.Exit(1)
	}

	os.Exit(m.Run())
}

// service is one running instance of the binary.
type service struct {
	baseURL string
	logs    *bytes.Buffer
	cmd     *exec.Cmd
}

// start runs the service on a free port with its own database, and stops it when the
// test ends.
func start(t *testing.T, env ...string) *service {
	t.Helper()

	address := fmt.Sprintf("127.0.0.1:%d", freePort(t))

	cmd := exec.Command(binary)
	cmd.Env = append(os.Environ(),
		"DB_PATH="+filepath.Join(t.TempDir(), "edh_stats.db"),
		"SERVER_ADDRESS="+address,
	)
	cmd.Env = append(cmd.Env, env...)

	logs := &bytes.Buffer{}
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("start the service: %v", err)
	}

	svc := &service{baseURL: "http://" + address, logs: logs, cmd: cmd}
	t.Cleanup(func() {
		svc.stop(t)
		if t.Failed() {
			t.Logf("service log:\n%s", logs.String())
		}
	})

	svc.waitHealthy(t)
	return svc
}

// stop asks the service to shut down the way the container runtime does, and insists
// that it does so cleanly: a non-zero exit means the shutdown path is broken.
func (s *service) stop(t *testing.T) {
	t.Helper()

	if err := s.cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Errorf("signal the service: %v", err)
		return
	}

	done := make(chan error, 1)
	go func() { done <- s.cmd.Wait() }()

	select {
	case err := <-done:
		if err != nil {
			t.Errorf("the service did not shut down cleanly: %v", err)
		}
	case <-time.After(20 * time.Second):
		_ = s.cmd.Process.Kill()
		t.Error("the service did not exit within 20s of SIGTERM")
	}
}

// waitHealthy polls /health until it answers.
//
// The poll cannot use do(), which fails the test on a transport error: a service that has
// just been started has not bound its port yet, and "connection refused" is the expected
// answer for the first few milliseconds.
func (s *service) waitHealthy(t *testing.T) {
	t.Helper()

	client := &http.Client{Timeout: time.Second}

	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		response, err := client.Get(s.baseURL + "/health")
		if err == nil {
			status := response.StatusCode
			_ = response.Body.Close()
			if status == http.StatusOK {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("the service did not become healthy; log:\n%s", s.logs.String())
}

// do sends a request and returns the status and the body.
func (s *service) do(t *testing.T, method, path, body string) (int, []byte) {
	t.Helper()

	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}

	request, err := http.NewRequest(method, s.baseURL+path, reader)
	if err != nil {
		t.Fatalf("build the request: %v", err)
	}
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()

	payload, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read the response body: %v", err)
	}
	return response.StatusCode, payload
}

// want checks the status and, when a fragment is given, that the body contains it.
func (s *service) want(t *testing.T, method, path, body string, status int, fragment string) []byte {
	t.Helper()

	gotStatus, payload := s.do(t, method, path, body)
	if gotStatus != status {
		t.Fatalf("%s %s = %d, want %d (body: %s)", method, path, gotStatus, status, payload)
	}
	if fragment != "" && !strings.Contains(string(payload), fragment) {
		t.Fatalf("%s %s body does not contain %q: %s", method, path, fragment, payload)
	}
	return payload
}

// TestLifecycle walks the service the way a client would: record decks, record a game,
// look at the statistics, fix a result, then clean up.
func TestLifecycle(t *testing.T) {
	svc := start(t)

	// An empty database answers with an empty list, not with null.
	if payload := svc.want(t, http.MethodGet, "/decks", "", http.StatusOK, "[]"); string(payload) != "[]\n" {
		t.Fatalf("an empty listing = %q, want []", payload)
	}

	svc.want(t, http.MethodPost, "/decks",
		`{"name":"Clement Bounce","owner":"Andrew Kornilov","type":"control"}`,
		http.StatusCreated, "Clement Bounce")
	svc.want(t, http.MethodPost, "/decks",
		`{"name":"Magar Spells","owner":"Ivan","type":"spells"}`,
		http.StatusCreated, "Magar Spells")

	// A duplicate is a conflict, and says so with a machine-readable code.
	svc.want(t, http.MethodPost, "/decks",
		`{"name":"Clement Bounce","owner":"Andrew Kornilov","type":"control"}`,
		http.StatusConflict, `"code":"deck_duplicate"`)

	// A game needs at least two participants.
	svc.want(t, http.MethodPost, "/games", `{"decks":[{"deck_id":1}]}`,
		http.StatusBadRequest, `"code":"validation_failed"`)

	// The whole game comes back, with names next to the identifiers.
	svc.want(t, http.MethodPost, "/games",
		`{"played_at":"2026-02-07","notes":"opps conceded","decks":[{"deck_id":1,"player":"Andrew Kornilov","result":"win"},{"deck_id":2}]}`,
		http.StatusCreated, `"deck_name":"Magar Spells"`)

	svc.want(t, http.MethodGet, "/games?limit=10", "", http.StatusOK, `"deck_name":"Clement Bounce"`)
	svc.want(t, http.MethodGet, "/games/1", "", http.StatusOK, `"played_at":"2026-02-07"`)
	svc.want(t, http.MethodGet, "/stats/decks", "", http.StatusOK,
		`{"deck_id":1,"name":"Clement Bounce","owner":"Andrew Kornilov","games":1,"wins":1,"losses":0,"draws":0}`)

	// The opponent's deck played the same game, but nothing was recorded for it.
	svc.want(t, http.MethodGet, "/stats/decks", "", http.StatusOK,
		`"name":"Magar Spells","owner":"Ivan","games":1,"wins":0,"losses":0,"draws":0`)

	// Fixing a result does not touch the other participations, and it shows up where a
	// client would look for it: in the statistics.
	svc.want(t, http.MethodPatch, "/games/1/decks/2",
		`{"player":"Ivan","result":"draw"}`, http.StatusOK, `"deck_id":2`)
	svc.want(t, http.MethodGet, "/stats/decks", "", http.StatusOK,
		`"name":"Magar Spells","owner":"Ivan","games":1,"wins":0,"losses":0,"draws":1`)

	// Clearing it is an empty string, not a missing field.
	svc.want(t, http.MethodPatch, "/games/1/decks/2", `{"result":""}`, http.StatusOK, `"deck_id":2`)
	svc.want(t, http.MethodGet, "/stats/decks", "", http.StatusOK,
		`"name":"Magar Spells","owner":"Ivan","games":1,"wins":0,"losses":0,"draws":0`)

	// The date of the game itself.
	svc.want(t, http.MethodPatch, "/games/1", `{"played_at":"2026-05-23"}`, http.StatusOK, `"played_at":"2026-05-23"`)

	// A deck that a game still refers to cannot be deleted: the database refuses, and
	// the service reports it as a conflict rather than as a failure.
	svc.want(t, http.MethodDelete, "/decks/1", "", http.StatusConflict, `"code":"deck_in_use"`)

	svc.want(t, http.MethodDelete, "/games/1", "", http.StatusNoContent, "")
	svc.want(t, http.MethodGet, "/games/1", "", http.StatusNotFound, `"code":"game_not_found"`)
	svc.want(t, http.MethodDelete, "/decks/1", "", http.StatusNoContent, "")
	svc.want(t, http.MethodDelete, "/decks/1", "", http.StatusNotFound, `"code":"deck_not_found"`)
}

// TestHealthcheckSubcommand covers the probe the container runs from inside itself.
func TestHealthcheckSubcommand(t *testing.T) {
	svc := start(t)

	probe := exec.Command(binary, "healthcheck")
	probe.Env = append(os.Environ(), "SERVER_ADDRESS="+strings.TrimPrefix(svc.baseURL, "http://"))
	probe.Stderr = os.Stderr

	if err := probe.Run(); err != nil {
		t.Errorf("the healthcheck subcommand failed against a healthy service: %v", err)
	}
}

// TestToken covers the optional bearer token: everything but /health needs it.
func TestToken(t *testing.T) {
	svc := start(t, "API_TOKEN=s3cret-token")

	svc.want(t, http.MethodGet, "/health", "", http.StatusOK, `"healthy"`)
	svc.want(t, http.MethodGet, "/decks", "", http.StatusUnauthorized, `"code":"unauthorized"`)

	request, err := http.NewRequest(http.MethodGet, svc.baseURL+"/decks", nil)
	if err != nil {
		t.Fatalf("build the request: %v", err)
	}
	request.Header.Set("Authorization", "Bearer s3cret-token")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("authorized request: %v", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode != http.StatusOK {
		t.Fatalf("an authorized request = %d, want %d", response.StatusCode, http.StatusOK)
	}
}

// TestUnknownPath is the API contract for a typo: 404, and 405 for a known path with the
// wrong method.
func TestUnknownPath(t *testing.T) {
	svc := start(t)

	svc.want(t, http.MethodGet, "/nonsense", "", http.StatusNotFound, "")
	if status, _ := svc.do(t, http.MethodDelete, "/decks", ""); status != http.StatusMethodNotAllowed {
		t.Errorf("DELETE /decks = %d, want %d", status, http.StatusMethodNotAllowed)
	}
}

// freePort asks the kernel for a port that is free right now.
func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer func() { _ = listener.Close() }()

	return listener.Addr().(*net.TCPAddr).Port
}
