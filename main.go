package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	instance := flag.String("instance", "https://fluxer.app", "Fluxer instance origin for discovery")
	base := flag.String("api-base", "", "override discovered API base (include /v1)")
	interval := flag.Duration("interval", 5*time.Second, "playback polling interval (minimum 2s)")
	dry := flag.Bool("dry-run", false, "print playback without contacting Fluxer")
	tokenFile := flag.String("token-file", "", "read Fluxer user session token from a local file")
	flag.Parse()
	if *interval < 2*time.Second || *interval > 30*time.Second {
		return fmt.Errorf("interval must be between 2s and 30s")
	}
	token := ""
	if !*dry {
		var err error
		token, err = readToken(*tokenFile)
		if err != nil {
			return err
		}
		if token == "" {
			return fmt.Errorf("set FLUXER_TOKEN or -token-file to your Fluxer user session token, or use -dry-run")
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	client := &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error { return http.ErrUseLastResponse }}
	if !*dry {
		if *base == "" {
			var err error
			*base, err = discover(ctx, client, *instance)
			if err != nil {
				return fmt.Errorf("discover API: %w", err)
			}
		}
		if err := secureURL(*base); err != nil {
			return err
		}
		*base = strings.TrimRight(*base, "/")
		if !strings.HasSuffix(*base, "/v1") {
			return fmt.Errorf("api-base must end with /v1")
		}
	}
	f := &fluxer{base: *base, token: token, client: client}
	last := ""
	owned := false
	refresh := time.Time{}
	retry := time.Time{}
	// A short expiry also removes stale status after a crash or forced termination.
	const ttl = 2 * time.Minute
	defer func() {
		if owned && !*dry {
			cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := f.set(cleanup, "", ttl); err != nil {
				log.Print("cleanup failed; status will expire")
			}
		}
	}()
	ticker := time.NewTicker(*interval)
	defer ticker.Stop()
	for {
		if time.Now().After(retry) {
			readCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			t, err := nowPlaying(readCtx)
			cancel()
			if err != nil {
				if ctx.Err() == nil {
					log.Print(err)
				}
			} else {
				text := statusText(t)
				if text != last || (owned && text != "" && time.Now().After(refresh)) {
					if *dry {
						log.Printf("now playing: %q", text)
						last = text
					} else if text != "" || owned {
						err = f.set(ctx, text, ttl)
						if err == nil {
							last = text
							owned = text != ""
							refresh = time.Now().Add(time.Minute)
							log.Printf("status updated: %q", text)
						} else {
							log.Print(err)
							retry = time.Now().Add(30 * time.Second)
							var api *apiError
							if errors.As(err, &api) {
								if api.code == 401 || api.code == 403 {
									return fmt.Errorf("Fluxer rejected credentials or status; check user session token/account access")
								}
								retry = time.Now().Add(api.wait)
							}
						}
					}
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func readToken(path string) (string, error) {
	if path == "" {
		return strings.TrimSpace(os.Getenv("FLUXER_TOKEN")), nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("token file must be a regular file")
	}
	if runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("token file must be accessible only to its owner (chmod 600 %q)", path)
	}
	if info.Size() > 4096 {
		return "", fmt.Errorf("token file is too large")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read token file: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}
