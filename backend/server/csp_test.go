package server

import (
	"strings"
	"testing"
)

func TestGenerateNonce(t *testing.T) {
	t.Run("returns valid base64", func(t *testing.T) {
		nonce, err := generateNonce()
		if err != nil {
			t.Fatalf("generateNonce() error: %v", err)
		}
		if nonce == "" {
			t.Fatal("generateNonce() returned empty string")
		}
		// 128 bits / 6 bits per base64 char ≈ 22 chars (unpadded)
		if len(nonce) < 20 || len(nonce) > 24 {
			t.Errorf("expected nonce length ~22, got %d (%q)", len(nonce), nonce)
		}
	})

	t.Run("each call is unique", func(t *testing.T) {
		seen := make(map[string]bool)
		for i := 0; i < 100; i++ {
			nonce, err := generateNonce()
			if err != nil {
				t.Fatalf("generateNonce() error on iteration %d: %v", i, err)
			}
			if seen[nonce] {
				t.Fatalf("duplicate nonce detected on iteration %d: %q", i, nonce)
			}
			seen[nonce] = true
		}
	})
}

func TestBuildCSP(t *testing.T) {
	hashes := []string{"'sha256-abc123'", "'sha256-def456'"}
	nonce := "testNonce123"

	t.Run("nonce mode has no unsafe-inline", func(t *testing.T) {
		csp := buildCSP(nonce, hashes)
		if strings.Contains(csp, "'unsafe-inline'") {
			t.Errorf("buildCSP should not contain 'unsafe-inline', got:\n%s", csp)
		}
	})

	t.Run("nonce mode contains nonce directive", func(t *testing.T) {
		csp := buildCSP(nonce, hashes)
		expected := "'nonce-testNonce123'"
		if !strings.Contains(csp, expected) {
			t.Errorf("buildCSP should contain %s, got:\n%s", expected, csp)
		}
	})

	t.Run("nonce mode contains report-uri", func(t *testing.T) {
		csp := buildCSP(nonce, hashes)
		if !strings.Contains(csp, "report-uri /api/csp-report") {
			t.Errorf("buildCSP should contain report-uri, got:\n%s", csp)
		}
	})

	t.Run("production CSP has no ws: in connect-src", func(t *testing.T) {
		csp := buildCSP(nonce, hashes)
		// Extract connect-src directive
		for _, part := range strings.Split(csp, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "connect-src") {
				if strings.Contains(part, " ws: ") || strings.HasSuffix(part, " ws:") {
					t.Errorf("production connect-src should not contain ws:, got: %s", part)
				}
				if strings.Contains(part, "data:") {
					t.Errorf("production connect-src should not contain data:, got: %s", part)
				}
				break
			}
		}
	})
}

func TestBuildCSPDev(t *testing.T) {
	hashes := []string{"'sha256-abc123'"}
	nonce := "devNonce999"

	t.Run("dev CSP allows ws: for HMR", func(t *testing.T) {
		csp := buildCSPDev(nonce, hashes)
		if !strings.Contains(csp, "ws:") {
			t.Errorf("dev CSP should contain ws: for HMR, got:\n%s", csp)
		}
	})

	t.Run("dev CSP has no unsafe-inline", func(t *testing.T) {
		csp := buildCSPDev(nonce, hashes)
		if strings.Contains(csp, "'unsafe-inline'") {
			t.Errorf("dev buildCSP should not contain 'unsafe-inline', got:\n%s", csp)
		}
	})
}

func TestBuildCSPLegacy(t *testing.T) {
	hashes := []string{"'sha256-abc123'"}

	t.Run("legacy CSP has unsafe-inline", func(t *testing.T) {
		csp := buildCSPLegacy(hashes)
		if !strings.Contains(csp, "'unsafe-inline'") {
			t.Errorf("legacy CSP should contain 'unsafe-inline', got:\n%s", csp)
		}
	})

	t.Run("legacy CSP has no nonce", func(t *testing.T) {
		csp := buildCSPLegacy(hashes)
		if strings.Contains(csp, "nonce-") {
			t.Errorf("legacy CSP should not contain nonce, got:\n%s", csp)
		}
	})
}
