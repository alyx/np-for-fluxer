package main

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
	"github.com/saltosystems/winrt-go/windows/media/control"
	"golang.org/x/sys/windows"
)

var roUninitialize = windows.NewLazySystemDLL("combase.dll").NewProc("RoUninitialize")

func nowPlayingWindows(ctx context.Context) (track, error) {
	if err := ctx.Err(); err != nil {
		return track{}, err
	}
	// COM initialization and all interface use must stay on the same OS thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.RoInitialize(1); err != nil { // RO_INIT_MULTITHREADED
		var oleErr *ole.OleError
		if !errors.As(err, &oleErr) || oleErr.Code() != 1 { // S_FALSE is success too.
			return track{}, fmt.Errorf("initialize WinRT: %w", err)
		}
	}
	defer roUninitialize.Call()

	op, err := requestMediaManager()
	if err != nil {
		return track{}, fmt.Errorf("request Windows media manager: %w", err)
	}
	result, err := awaitWinRT(ctx, op)
	if err != nil {
		return track{}, fmt.Errorf("get Windows media manager: %w", err)
	}
	manager := (*control.GlobalSystemMediaTransportControlsSessionManager)(result)
	defer manager.Release()
	sessions, err := manager.GetSessions()
	if err != nil {
		return track{}, fmt.Errorf("list Windows media sessions: %w", err)
	}
	defer sessions.Release()
	size, err := sessions.GetSize()
	if err != nil {
		return track{}, fmt.Errorf("count Windows media sessions: %w", err)
	}
	for i := uint32(0); i < size; i++ {
		if err := ctx.Err(); err != nil {
			return track{}, err
		}
		ptr, err := sessions.GetAt(i)
		if err != nil {
			return track{}, fmt.Errorf("get Windows media session: %w", err)
		}
		session := (*control.GlobalSystemMediaTransportControlsSession)(ptr)
		t, matched, err := readWindowsSession(ctx, session)
		session.Release()
		if err != nil || matched {
			return t, err
		}
	}
	return track{}, nil
}

func readWindowsSession(ctx context.Context, session *control.GlobalSystemMediaTransportControlsSession) (track, bool, error) {
	id, err := session.GetSourceAppUserModelId()
	if err != nil {
		return track{}, false, fmt.Errorf("read media app ID: %w", err)
	}
	if !strings.Contains(strings.ToLower(id), "spotify") {
		return track{}, false, nil
	}
	info, err := session.GetPlaybackInfo()
	if err != nil {
		return track{}, true, fmt.Errorf("read Spotify playback info: %w", err)
	}
	defer info.Release()
	status, err := info.GetPlaybackStatus()
	if err != nil {
		return track{}, true, fmt.Errorf("read Spotify playback status: %w", err)
	}
	if status != control.GlobalSystemMediaTransportControlsSessionPlaybackStatusPlaying {
		return track{}, true, nil
	}
	op, err := session.TryGetMediaPropertiesAsync()
	if err != nil {
		return track{}, true, fmt.Errorf("request Spotify metadata: %w", err)
	}
	result, err := awaitWinRT(ctx, op)
	if err != nil {
		return track{}, true, fmt.Errorf("read Spotify metadata: %w", err)
	}
	media := (*control.GlobalSystemMediaTransportControlsSessionMediaProperties)(result)
	defer media.Release()
	title, err := media.GetTitle()
	if err != nil {
		return track{}, true, fmt.Errorf("read Spotify title: %w", err)
	}
	artist, err := media.GetArtist()
	if err != nil {
		return track{}, true, fmt.Errorf("read Spotify artist: %w", err)
	}
	return track{Title: title, Artist: artist, Playing: true}, true, nil
}

// The generated static helper omits Release on its activation factory. Own that
// reference here; the remaining media calls use the generated bindings directly.
func requestMediaManager() (*foundation.IAsyncOperation, error) {
	factory, err := ole.RoGetActivationFactory("Windows.Media.Control.GlobalSystemMediaTransportControlsSessionManager",
		ole.NewGUID(control.GUIDiGlobalSystemMediaTransportControlsSessionManagerStatics))
	if err != nil {
		return nil, err
	}
	defer factory.Release()
	type managerStaticsVtbl struct {
		ole.IInspectableVtbl
		RequestAsync uintptr
	}
	var op *foundation.IAsyncOperation
	hr, _, _ := syscall.SyscallN((*managerStaticsVtbl)(unsafe.Pointer(factory.RawVTable)).RequestAsync,
		uintptr(unsafe.Pointer(factory)), uintptr(unsafe.Pointer(&op)))
	if int32(hr) < 0 {
		return nil, ole.NewError(hr)
	}
	return op, nil
}

func awaitWinRT(ctx context.Context, op *foundation.IAsyncOperation) (unsafe.Pointer, error) {
	defer op.Release()
	unknown, err := op.QueryInterface(ole.NewGUID(foundation.GUIDIAsyncInfo))
	if err != nil {
		return nil, err
	}
	defer unknown.Release()
	info := (*foundation.IAsyncInfo)(unsafe.Pointer(unknown))
	defer info.Close()
	if err := waitWinRT(ctx, info); err != nil {
		return nil, err
	}
	result, err := op.GetResults()
	if err == nil && result == nil {
		err = fmt.Errorf("WinRT operation returned no result")
	}
	return result, err
}

type winRTAsyncInfo interface {
	GetStatus() (foundation.AsyncStatus, error)
	GetErrorCode() (foundation.HResult, error)
	Cancel() error
}

func waitWinRT(ctx context.Context, info winRTAsyncInfo) error {
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		if err := ctx.Err(); err != nil {
			_ = info.Cancel()
			return err
		}
		status, err := info.GetStatus()
		if err != nil {
			return err
		}
		switch status {
		case foundation.AsyncStatusCompleted:
			return nil
		case foundation.AsyncStatusCanceled:
			return context.Canceled
		case foundation.AsyncStatusError:
			code, err := info.GetErrorCode()
			if err != nil {
				return err
			}
			return ole.NewError(uintptr(uint32(code.Value)))
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
}
