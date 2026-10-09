package deadline

import (
	"context"
	"errors"
	"testing"
	"time"
)

// stuckCtx has a deadline in the past but has not been cancelled, like a
// timer context whose timer could not fire.
type stuckCtx struct {
	context.Context
	d time.Time
}

func (c stuckCtx) Deadline() (time.Time, bool) { return c.d, true }

func TestErr(t *testing.T) {
	if err := Err(context.Background()); err != nil {
		t.Errorf("background: %v", err)
	}

	future, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	if err := Err(future); err != nil {
		t.Errorf("future deadline: %v", err)
	}

	cancelled, cancel2 := context.WithCancel(context.Background())
	cancel2()
	if err := Err(cancelled); !errors.Is(err, context.Canceled) {
		t.Errorf("cancelled: %v", err)
	}

	stuck := stuckCtx{context.Background(), time.Now().Add(-time.Millisecond)}
	if err := Err(stuck); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("past deadline not yet noticed: %v", err)
	}
}
