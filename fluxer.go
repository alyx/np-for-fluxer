package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type fluxer struct {
	base, token string
	client      *http.Client
}

func secureURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid endpoint URL")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return fmt.Errorf("endpoint requires HTTPS (HTTP allowed only on loopback)")
	}
	return nil
}

func discover(ctx context.Context, client *http.Client, instance string) (string, error) {
	if err := secureURL(instance); err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", strings.TrimRight(instance, "/")+"/.well-known/fluxer", nil)
	if err != nil {
		return "", err
	}
	// The hosted instance redirects discovery to api.fluxer.app. This request has
	// no credential; keep the status client itself configured to refuse redirects.
	discoveryClient := *client
	discoveryClient.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many discovery redirects")
		}
		if err := secureURL(next.URL.String()); err != nil {
			return fmt.Errorf("unsafe discovery redirect: %w", err)
		}
		if via[0].URL.Scheme == "https" && next.URL.Scheme != "https" {
			return fmt.Errorf("discovery redirect downgraded HTTPS")
		}
		if next.URL.Path != "/.well-known/fluxer" {
			return fmt.Errorf("discovery redirect changed path")
		}
		return nil
	}
	res, err := discoveryClient.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", fmt.Errorf("discovery HTTP %d", res.StatusCode)
	}
	var d struct {
		Endpoints struct {
			API string `json:"api_public"`
		} `json:"endpoints"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&d); err != nil {
		return "", err
	}
	base := strings.TrimRight(d.Endpoints.API, "/")
	if err := secureURL(base); err != nil {
		return "", err
	}
	if !strings.HasSuffix(base, "/v1") {
		base += "/v1"
	}
	return base, nil
}

type apiError struct {
	code int
	wait time.Duration
}

func (e *apiError) Error() string { return fmt.Sprintf("Fluxer HTTP %d", e.code) }

func (f *fluxer) set(ctx context.Context, text string, ttl time.Duration) error {
	var status any
	if text != "" {
		status = map[string]any{"text": text, "emoji_name": "🎵", "expires_at": time.Now().Add(ttl).UTC().Format(time.RFC3339)}
	}
	body, _ := json.Marshal(map[string]any{"custom_status": status})
	req, err := http.NewRequestWithContext(ctx, "PATCH", f.base+"/users/@me/settings", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", f.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "np-for-fluxer/0.1")
	res, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 200 && res.StatusCode < 300 {
		return nil
	}
	e := &apiError{code: res.StatusCode, wait: 30 * time.Second}
	if seconds, err := strconv.ParseFloat(res.Header.Get("Retry-After"), 64); err == nil && seconds > 0 {
		e.wait = time.Duration(seconds * float64(time.Second))
	} else if date, err := http.ParseTime(res.Header.Get("Retry-After")); err == nil && time.Until(date) > 0 {
		e.wait = time.Until(date)
	}
	if res.StatusCode == 429 {
		var b struct {
			Retry float64 `json:"retry_after"`
		}
		if json.NewDecoder(io.LimitReader(res.Body, 65536)).Decode(&b) == nil && b.Retry > 0 {
			if d := time.Duration(b.Retry * float64(time.Second)); d > e.wait {
				e.wait = d
			}
		}
	}
	return e
}

func statusText(t track) string {
	if !t.Playing || strings.TrimSpace(t.Title) == "" {
		return ""
	}
	s := strings.Join(strings.Fields(t.Title), " ")
	if a := strings.Join(strings.Fields(t.Artist), " "); a != "" {
		s += " — " + a
	}
	r := []rune(s)
	if len(r) > 128 {
		s = string(r[:127]) + "…"
	}
	return s
}
