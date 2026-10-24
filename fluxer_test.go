package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestStatusText(t *testing.T) {
	if got := statusText(track{Title: " A\nSong ", Artist: " An Artist ", Playing: true}); got != "A Song — An Artist" {
		t.Fatal(got)
	}
	if got := statusText(track{Title: "Paused"}); got != "" {
		t.Fatal(got)
	}
	got := statusText(track{Title: strings.Repeat("🎸", 200), Playing: true})
	if !utf8.ValidString(got) || len([]rune(got)) != 128 {
		t.Fatal("invalid Unicode truncation")
	}
}

func TestFluxerStatusLifecycle(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PATCH" || r.URL.Path != "/v1/users/@me/settings" || r.Header.Get("Authorization") != "test-token" {
			t.Error("incorrect request")
		}
		var body map[string]json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if len(body) != 1 {
			t.Error("must only change custom_status")
		}
		if calls == 1 {
			var status struct {
				Text    string `json:"text"`
				Expires string `json:"expires_at"`
			}
			if err := json.Unmarshal(body["custom_status"], &status); err != nil {
				t.Error(err)
			}
			expiry, err := time.Parse(time.RFC3339, status.Expires)
			if err != nil || !expiry.After(time.Now()) || status.Text != "Song — Artist" {
				t.Error("invalid status payload")
			}
		} else if string(body["custom_status"]) != "null" {
			t.Error("must clear with null")
		}
		w.WriteHeader(200)
	}))
	defer server.Close()
	f := fluxer{server.URL + "/v1", "test-token", server.Client()}
	for _, text := range []string{"Song — Artist", ""} {
		if err := f.set(context.Background(), text, time.Minute); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRateLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "45")
		w.WriteHeader(429)
		_, _ = w.Write([]byte(`{"retry_after":60}`))
	}))
	defer server.Close()
	f := fluxer{server.URL, "token", server.Client()}
	err := f.set(context.Background(), "song", time.Minute)
	var api *apiError
	if !errors.As(err, &api) || api.wait != time.Minute {
		t.Fatalf("unexpected retry: %v", err)
	}
}

func TestDiscovery(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/fluxer" {
			t.Error(r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"endpoints":{"api_public":"https://example.test/api"}}`))
	}))
	defer server.Close()
	base, err := discover(context.Background(), server.Client(), server.URL)
	if err != nil || base != "https://example.test/api/v1" {
		t.Fatalf("%s %v", base, err)
	}
	for _, raw := range []string{"http://example.com", "https://user:pass@example.com", "https://example.com?token=secret"} {
		if secureURL(raw) == nil {
			t.Errorf("accepted %s", raw)
		}
	}
}

func TestDiscoveryRedirect(t *testing.T) {
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/.well-known/fluxer" || r.Header.Get("Authorization") != "" {
			t.Error("unexpected discovery request")
		}
		_, _ = w.Write([]byte(`{"endpoints":{"api_public":"https://api.fluxer.app"}}`))
	}))
	defer destination.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL+"/.well-known/fluxer", http.StatusPermanentRedirect)
	}))
	defer source.Close()
	client := source.Client()
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }
	base, err := discover(context.Background(), client, source.URL)
	if err != nil || base != "https://api.fluxer.app/v1" {
		t.Fatalf("%s %v", base, err)
	}
}

func TestTokenFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(path, []byte("  token-value\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FLUXER_TOKEN", "different-token")
	got, err := readToken(path)
	if err != nil || got != "token-value" {
		t.Fatalf("token file precedence: %q %v", got, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(path, 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := readToken(path); err == nil {
			t.Fatal("accepted a token file readable by other users")
		}
	}
}
