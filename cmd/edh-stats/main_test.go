package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthcheckURL(t *testing.T) {
	tests := []struct {
		address string
		want    string
	}{
		{address: "", want: "http://127.0.0.1:8080/health"},
		{address: ":8080", want: "http://127.0.0.1:8080/health"},
		{address: "8080", want: "http://127.0.0.1:8080/health"},
		{address: "0.0.0.0:9000", want: "http://127.0.0.1:9000/health"},
		{address: "[::]:9000", want: "http://127.0.0.1:9000/health"},
		{address: "127.0.0.1:1234", want: "http://127.0.0.1:1234/health"},
	}

	for _, test := range tests {
		t.Run(test.address, func(t *testing.T) {
			t.Setenv("SERVER_ADDRESS", test.address)

			if got := healthcheckURL(); got != test.want {
				t.Errorf("healthcheckURL() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestHealthcheck(t *testing.T) {
	// serve starts a stub service and points the probe at it.
	serve := func(t *testing.T, status int, body string) {
		t.Helper()

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(body))
		}))
		t.Cleanup(server.Close)

		t.Setenv("SERVER_ADDRESS", strings.TrimPrefix(server.URL, "http://"))
	}

	t.Run("healthy service", func(t *testing.T) {
		serve(t, http.StatusOK, `{"status":"healthy"}`)

		if err := healthcheck(); err != nil {
			t.Errorf("healthcheck() = %v, want nil", err)
		}
	})

	t.Run("unhealthy service", func(t *testing.T) {
		serve(t, http.StatusServiceUnavailable, `{"status":"unhealthy"}`)

		if err := healthcheck(); err == nil {
			t.Error("healthcheck() = nil, want an error")
		}
	})

	// A 200 that does not report a healthy status must not pass either.
	t.Run("unexpected body", func(t *testing.T) {
		serve(t, http.StatusOK, `{"status":"degraded"}`)

		if err := healthcheck(); err == nil {
			t.Error("healthcheck() = nil, want an error")
		}
	})

	t.Run("nothing listening", func(t *testing.T) {
		// Bind a port and release it again, so that nothing answers the probe.
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatalf("listen: %v", err)
		}
		address := listener.Addr().String()
		if err := listener.Close(); err != nil {
			t.Fatalf("close listener: %v", err)
		}

		t.Setenv("SERVER_ADDRESS", address)

		if err := healthcheck(); err == nil {
			t.Error("healthcheck() = nil, want an error")
		}
	})
}
