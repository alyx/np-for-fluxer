package main

import (
	"context"
	"fmt"
	"runtime"
)

type track struct {
	Title   string `json:"title"`
	Artist  string `json:"artist"`
	Playing bool   `json:"playing"`
}

func nowPlaying(ctx context.Context) (track, error) {
	switch runtime.GOOS {
	case "windows":
		return nowPlayingWindows(ctx)
	case "darwin":
		return nowPlayingMac(ctx)
	case "linux":
		return nowPlayingDBus(ctx)
	default:
		return track{}, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}
