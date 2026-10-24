package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

var macFrameworks struct {
	sync.Once
	err error
}

func loadMacFrameworks() error {
	macFrameworks.Do(func() {
		// Keep framework handles open for the lifetime of their Objective-C classes.
		for _, name := range []string{"Foundation", "AppKit"} {
			_, err := purego.Dlopen("/System/Library/Frameworks/"+name+".framework/"+name, purego.RTLD_NOW|purego.RTLD_GLOBAL)
			if err != nil {
				macFrameworks.err = fmt.Errorf("load %s: %w", name, err)
				return
			}
		}
	})
	return macFrameworks.err
}

func nowPlayingMac(ctx context.Context) (track, error) {
	if err := ctx.Err(); err != nil {
		return track{}, err
	}
	if err := loadMacFrameworks(); err != nil {
		return track{}, err
	}
	// Autoreleased descriptors and strings must be drained on their creating thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := macSend[objc.ID](macClass("NSAutoreleasePool"), "new")
	defer macSend[objc.ID](pool, "drain")
	apps := macSend[objc.ID](macClass("NSRunningApplication"), "runningApplicationsWithBundleIdentifier:", macString("com.spotify.client"))
	if macSend[uint64](apps, "count") == 0 {
		return track{}, nil
	}
	app := macSend[objc.ID](apps, "objectAtIndex:", uint64(0))
	pid := macSend[int32](app, "processIdentifier")
	if pid <= 0 || macSend[bool](app, "isTerminated") {
		return track{}, nil
	}
	// Address an existing process, never a bundle ID that could launch Spotify.
	target := macSend[objc.ID](macClass("NSAppleEventDescriptor"), "descriptorWithProcessIdentifier:", pid)
	root := macSend[objc.ID](macClass("NSAppleEventDescriptor"), "nullDescriptor")
	state, err := macGetProperty(ctx, target, macProperty(root, "pPlS"))
	if err != nil {
		return macPlaybackError(err)
	}
	if macSend[uint32](state, "enumCodeValue") != macCode("kPSP") {
		return track{}, nil
	}
	currentTrack := macProperty(root, "pTrk")
	title, err := macGetProperty(ctx, target, macProperty(currentTrack, "pnam"))
	if err != nil {
		return macPlaybackError(err)
	}
	artist, err := macGetProperty(ctx, target, macProperty(currentTrack, "pArt"))
	if err != nil {
		return macPlaybackError(err)
	}
	return track{
		Title:   macGoString(macSend[objc.ID](title, "stringValue")),
		Artist:  macGoString(macSend[objc.ID](artist, "stringValue")),
		Playing: true,
	}, nil
}

// Apple Event object specifier: property <code> of <container>.
func macProperty(container objc.ID, code string) objc.ID {
	class := macClass("NSAppleEventDescriptor")
	record := macSend[objc.ID](class, "recordDescriptor")
	for _, field := range []struct {
		key   string
		value objc.ID
	}{
		{"want", macSend[objc.ID](class, "descriptorWithTypeCode:", macCode("prop"))},
		{"from", container},
		{"form", macSend[objc.ID](class, "descriptorWithEnumCode:", macCode("prop"))},
		{"seld", macSend[objc.ID](class, "descriptorWithTypeCode:", macCode(code))},
	} {
		macSend[objc.ID](record, "setDescriptor:forKeyword:", field.value, macCode(field.key))
	}
	return macSend[objc.ID](record, "coerceToDescriptorType:", macCode("obj "))
}

func macGetProperty(ctx context.Context, target, property objc.ID) (objc.ID, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	// Native sends are synchronous. Bound cancellation latency to two seconds and
	// shorten each send to the remaining deadline; never leave a worker behind.
	timeout := 2 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < timeout {
			timeout = remaining
		}
	}
	if timeout <= 0 {
		return 0, context.DeadlineExceeded
	}
	if target == 0 || property == 0 {
		return 0, fmt.Errorf("create Spotify Apple Event descriptor")
	}
	event := macSend[objc.ID](macClass("NSAppleEventDescriptor"),
		"appleEventWithEventClass:eventID:targetDescriptor:returnID:transactionID:",
		macCode("core"), macCode("getd"), target, int16(-1), int32(0))
	macSend[objc.ID](event, "setParamDescriptor:forKeyword:", property, macCode("----"))
	var nativeError objc.ID
	// kAEWaitReply | kAECanInteract permits the normal Automation consent flow.
	reply := macSend[objc.ID](event, "sendEventWithOptions:timeout:error:", uint64(3|0x20), timeout.Seconds(), &nativeError)
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if reply == 0 {
		if nativeError != 0 {
			return 0, &macEventError{
				code:    macSend[int64](nativeError, "code"),
				message: macGoString(macSend[objc.ID](nativeError, "localizedDescription")),
			}
		}
		return 0, fmt.Errorf("Spotify Apple Event returned no reply")
	}
	errDesc := macSend[objc.ID](reply, "paramDescriptorForKeyword:", macCode("errn"))
	if code := macSend[int32](errDesc, "int32Value"); code != 0 {
		message := macSend[objc.ID](reply, "paramDescriptorForKeyword:", macCode("errs"))
		return 0, &macEventError{code: int64(code), message: macGoString(macSend[objc.ID](message, "stringValue"))}
	}
	result := macSend[objc.ID](reply, "paramDescriptorForKeyword:", macCode("----"))
	if result == 0 {
		return 0, fmt.Errorf("Spotify Apple Event reply is missing its result")
	}
	return result, nil
}

type macEventError struct {
	code    int64
	message string
}

func (e *macEventError) Error() string {
	if e.code == -1743 {
		return "Spotify Apple Events access denied (-1743); allow Automation access to Spotify in System Settings > Privacy & Security > Automation"
	}
	return fmt.Sprintf("Spotify Apple Event failed (%d): %s", e.code, e.message)
}

func macPlaybackError(err error) (track, error) {
	var nativeError *macEventError
	if errors.As(err, &nativeError) && (nativeError.code == -600 || nativeError.code == -609) {
		return track{}, nil // Spotify exited between discovery and the request.
	}
	return track{}, err
}

func macClass(name string) objc.ID { return objc.ID(objc.GetClass(name)) }

func macSend[T any](object objc.ID, selector string, args ...any) T {
	return objc.Send[T](object, objc.RegisterName(selector), args...)
}

func macString(value string) objc.ID {
	return macSend[objc.ID](macClass("NSString"), "stringWithUTF8String:", value)
}

func macGoString(value objc.ID) string {
	if value == 0 {
		return ""
	}
	return macSend[string](value, "UTF8String")
}

func macCode(value string) uint32 {
	if len(value) != 4 {
		panic("Apple Event codes must be four bytes")
	}
	return uint32(value[0])<<24 | uint32(value[1])<<16 | uint32(value[2])<<8 | uint32(value[3])
}
