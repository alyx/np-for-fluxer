package main

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/go-ole/go-ole"
	"github.com/saltosystems/winrt-go/windows/foundation"
)

type fakeAsyncInfo struct {
	statuses []foundation.AsyncStatus
	err      error
	canceled bool
}

func (f *fakeAsyncInfo) GetStatus() (foundation.AsyncStatus, error) {
	status := f.statuses[0]
	if len(f.statuses) > 1 {
		f.statuses = f.statuses[1:]
	}
	return status, f.err
}

func (*fakeAsyncInfo) GetErrorCode() (foundation.HResult, error) {
	return foundation.HResult{Value: -2147467259}, nil // E_FAIL
}

func (f *fakeAsyncInfo) Cancel() error {
	f.canceled = true
	return nil
}

func TestWaitWinRT(t *testing.T) {
	t.Run("completion", func(t *testing.T) {
		info := &fakeAsyncInfo{statuses: []foundation.AsyncStatus{foundation.AsyncStatusStarted, foundation.AsyncStatusCompleted}}
		if err := waitWinRT(context.Background(), info); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("deadline cancels native operation", func(t *testing.T) {
		info := &fakeAsyncInfo{statuses: []foundation.AsyncStatus{foundation.AsyncStatusStarted}}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if err := waitWinRT(ctx, info); !errors.Is(err, context.DeadlineExceeded) || !info.canceled {
			t.Fatalf("error=%v, canceled=%v", err, info.canceled)
		}
	})
	t.Run("native cancellation", func(t *testing.T) {
		info := &fakeAsyncInfo{statuses: []foundation.AsyncStatus{foundation.AsyncStatusCanceled}}
		if err := waitWinRT(context.Background(), info); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	})
	t.Run("native error", func(t *testing.T) {
		info := &fakeAsyncInfo{statuses: []foundation.AsyncStatus{foundation.AsyncStatusError}}
		var oleErr *ole.OleError
		if err := waitWinRT(context.Background(), info); !errors.As(err, &oleErr) || oleErr.Code() != 0x80004005 {
			t.Fatal(err)
		}
	})
	t.Run("status error", func(t *testing.T) {
		want := errors.New("status unavailable")
		info := &fakeAsyncInfo{statuses: []foundation.AsyncStatus{foundation.AsyncStatusStarted}, err: want}
		if err := waitWinRT(context.Background(), info); !errors.Is(err, want) {
			t.Fatal(err)
		}
	})
}

func TestWindowsMediaIntegration(t *testing.T) {
	if os.Getenv("NP_TEST_WINDOWS_MEDIA") != "1" {
		t.Skip("set NP_TEST_WINDOWS_MEDIA=1 to read the current desktop's media sessions")
	}
	for i := 0; i < 3; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		got, err := nowPlaying(ctx)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("poll %d: playing=%v, title present=%v, artist present=%v", i+1, got.Playing, got.Title != "", got.Artist != "")
	}
}

func TestWindowsReaderCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := nowPlayingWindows(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation before accessing WinRT, got %v", err)
	}
}
