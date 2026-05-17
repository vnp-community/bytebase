package transport

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestBufconnTransport_RoundTrip(t *testing.T) {
	tr := NewBufconnTransport()
	defer tr.Close()

	// Start a simple HTTP server on the bufconn listener.
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("pong"))
	})
	go func() {
		srv := &http.Server{Handler: mux}
		_ = srv.Serve(tr.Listener())
	}()

	// Make a request via the bufconn HTTP client.
	resp, err := tr.HTTPClient().Get(tr.BaseURL() + "/ping")
	if err != nil {
		t.Fatalf("GET /ping error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("got status %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(body)) != "pong" {
		t.Fatalf("got body %q, want \"pong\"", body)
	}
}

func TestBufconnTransport_BaseURL(t *testing.T) {
	tr := NewBufconnTransport()
	defer tr.Close()

	if tr.BaseURL() != "http://bufconn" {
		t.Fatalf("BaseURL = %q, want %q", tr.BaseURL(), "http://bufconn")
	}
}

func TestBufconnTransport_DoubleClose(t *testing.T) {
	tr := NewBufconnTransport()
	if err := tr.Close(); err != nil {
		t.Fatalf("first Close error: %v", err)
	}
	// Second close should be safe.
	if err := tr.Close(); err != nil {
		t.Fatalf("second Close error: %v", err)
	}
}
