package main

import (
	"testing"

	"github.com/godbus/dbus/v5"
)

func TestTrackFromMPRIS(t *testing.T) {
	props := map[string]dbus.Variant{
		"PlaybackStatus": dbus.MakeVariant("Playing"),
		"Metadata": dbus.MakeVariant(map[string]dbus.Variant{
			"xesam:title":  dbus.MakeVariant("Song\nTitle"),
			"xesam:artist": dbus.MakeVariant([]string{"Artist A", "Artist B"}),
		}),
	}
	got, err := trackFromMPRIS(props)
	if err != nil || got != (track{Title: "Song\nTitle", Artist: "Artist A, Artist B", Playing: true}) {
		t.Fatalf("got %+v, %v", got, err)
	}
	for _, status := range []string{"Paused", "Stopped"} {
		got, err = trackFromMPRIS(map[string]dbus.Variant{"PlaybackStatus": dbus.MakeVariant(status)})
		if err != nil || got != (track{}) {
			t.Fatalf("%s: got %+v, %v", status, got, err)
		}
	}
	props["Metadata"] = dbus.MakeVariant(map[string]dbus.Variant{})
	got, err = trackFromMPRIS(props)
	if err != nil || got.Title != "" || got.Artist != "" {
		t.Fatalf("missing metadata fields: %+v, %v", got, err)
	}
	props["Metadata"] = dbus.MakeVariant("invalid")
	if _, err := trackFromMPRIS(props); err == nil {
		t.Fatal("expected invalid metadata error")
	}
	if _, err := trackFromMPRIS(nil); err == nil {
		t.Fatal("expected missing playback status error")
	}
}
