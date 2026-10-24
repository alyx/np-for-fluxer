package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/godbus/dbus/v5"
)

func nowPlayingDBus(ctx context.Context) (track, error) {
	// A private connection per poll also recovers from a restarted session bus.
	conn, err := dbus.ConnectSessionBus(dbus.WithContext(ctx))
	if err != nil {
		return track{}, fmt.Errorf("connect to desktop session D-Bus: %w", err)
	}
	defer conn.Close()
	var props map[string]dbus.Variant
	err = conn.Object("org.mpris.MediaPlayer2.spotify", "/org/mpris/MediaPlayer2").
		CallWithContext(ctx, "org.freedesktop.DBus.Properties.GetAll", dbus.FlagNoAutoStart,
			"org.mpris.MediaPlayer2.Player").Store(&props)
	if err != nil {
		var busErr dbus.Error
		if errors.As(err, &busErr) && (busErr.Name == "org.freedesktop.DBus.Error.ServiceUnknown" ||
			busErr.Name == "org.freedesktop.DBus.Error.NameHasNoOwner") {
			return track{}, nil // Spotify is closed.
		}
		return track{}, fmt.Errorf("read Spotify MPRIS properties: %w", err)
	}
	return trackFromMPRIS(props)
}

func trackFromMPRIS(props map[string]dbus.Variant) (track, error) {
	status, ok := props["PlaybackStatus"].Value().(string)
	if !ok {
		return track{}, fmt.Errorf("invalid MPRIS PlaybackStatus")
	}
	if status != "Playing" {
		return track{}, nil
	}
	metadata, ok := props["Metadata"].Value().(map[string]dbus.Variant)
	if !ok {
		return track{}, fmt.Errorf("invalid MPRIS Metadata")
	}
	title, _ := metadata["xesam:title"].Value().(string)
	artists, _ := metadata["xesam:artist"].Value().([]string)
	return track{Title: strings.TrimSpace(title), Artist: strings.Join(artists, ", "), Playing: true}, nil
}
