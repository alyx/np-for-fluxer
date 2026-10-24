//go:build !windows

package main

import (
	"context"
	"fmt"
)

func nowPlayingWindows(context.Context) (track, error) {
	return track{}, fmt.Errorf("Windows media sessions are only available on Windows")
}
