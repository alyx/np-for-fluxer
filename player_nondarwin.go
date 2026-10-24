//go:build !darwin

package main

import (
	"context"
	"fmt"
)

func nowPlayingMac(context.Context) (track, error) {
	return track{}, fmt.Errorf("Apple Events are only available on macOS")
}
