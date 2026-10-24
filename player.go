package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf16"
)

type track struct {
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Playing bool   `json:"playing"`
}

func nowPlaying(ctx context.Context) (track, error) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		return nowPlayingWindows(ctx)
	case "darwin":
		cmd = exec.CommandContext(ctx, "osascript", "-l", "JavaScript", "-e", macScript)
	case "linux":
		return nowPlayingDBus(ctx)
	default:
		return track{}, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return track{}, fmt.Errorf("Spotify reader failed (ensure desktop Spotify is running and media access is allowed): %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	var t track
	if err := json.Unmarshal(out, &t); err != nil {
		return t, fmt.Errorf("decode playback: %w", err)
	}
	return t, nil
}

const macScript = `function run() {
  const spotify = Application('com.spotify.client');
  if (!spotify.running() || spotify.playerState() !== 'playing') return JSON.stringify({playing:false});
  const t = spotify.currentTrack();
  return JSON.stringify({playing:true,title:t.name(),artist:t.artist()});
}`

