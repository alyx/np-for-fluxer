package main

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ebitengine/purego/objc"
)

func TestMacAppleEventDescriptors(t *testing.T) {
	if err := loadMacFrameworks(); err != nil {
		t.Fatal(err)
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := macSend[objc.ID](macClass("NSAutoreleasePool"), "new")
	defer macSend[objc.ID](pool, "drain")
	// Exercise the native UTF-8 conversion, including supplementary characters.
	want := "Björk — 東京 🎵"
	text := macSend[objc.ID](macClass("NSAppleEventDescriptor"), "descriptorWithString:", macString(want))
	if got := macGoString(macSend[objc.ID](text, "stringValue")); got != want {
		t.Fatalf("text round trip: got %q", got)
	}
	root := macSend[objc.ID](macClass("NSAppleEventDescriptor"), "nullDescriptor")
	title := macProperty(macProperty(root, "pTrk"), "pnam")
	if got := macSend[uint32](title, "descriptorType"); got != macCode("obj ") {
		t.Fatalf("expected object specifier, got %08x", got)
	}
	for _, spec := range []struct {
		object objc.ID
		code   string
	}{
		{title, "pnam"},
		{macSend[objc.ID](title, "descriptorForKeyword:", macCode("from")), "pTrk"},
	} {
		key := macSend[objc.ID](spec.object, "descriptorForKeyword:", macCode("seld"))
		if got := macSend[uint32](key, "typeCodeValue"); got != macCode(spec.code) {
			t.Fatalf("property code: got %08x, want %q", got, spec.code)
		}
		form := macSend[objc.ID](spec.object, "descriptorForKeyword:", macCode("form"))
		if got := macSend[uint32](form, "enumCodeValue"); got != macCode("prop") {
			t.Fatalf("expected property key form, got %08x", got)
		}
	}
}

func TestMacPlaybackErrors(t *testing.T) {
	for _, code := range []int64{-600, -609} {
		got, err := macPlaybackError(&macEventError{code: code})
		if err != nil || got != (track{}) {
			t.Fatalf("Spotify exit %d: %+v, %v", code, got, err)
		}
	}
	denied := &macEventError{code: -1743}
	if _, err := macPlaybackError(denied); !errors.Is(err, denied) || !strings.Contains(err.Error(), "Automation") {
		t.Fatalf("expected actionable permission error, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nowPlayingMac(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := macGetProperty(ctx, 0, 0); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestMacMediaIntegration(t *testing.T) {
	if os.Getenv("NP_TEST_MAC_MEDIA") != "1" {
		t.Skip("set NP_TEST_MAC_MEDIA=1 to read Spotify through Apple Events")
	}
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		got, err := nowPlayingMac(ctx)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("poll %d: playing=%v, title present=%v, artist present=%v", i+1, got.Playing, got.Title != "", got.Artist != "")
	}
}
