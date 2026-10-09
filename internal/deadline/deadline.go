// Package deadline tells solvers when to stop.
package deadline

import (
	"context"
	"time"
)

// Err returns ctx.Err(), or context.DeadlineExceeded once the context's
// deadline has passed even if the context has not noticed yet. Under
// GOOS=js (WebAssembly in a browser) there is one thread and no preemption,
// so the timer behind context.WithTimeout cannot fire while a solver keeps
// computing; reading the clock makes deadlines work there too.
func Err(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if d, ok := ctx.Deadline(); ok && !time.Now().Before(d) {
		return context.DeadlineExceeded
	}
	return nil
}
