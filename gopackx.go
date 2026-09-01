// Package gopackx provides a high-level API for 3D variable-sized bin packing.
//
// The [Pack] function is the primary entry point. Given a set of available box
// types and items to pack, it selects the appropriate solver and returns an
// optimal or near-optimal packing that minimizes the number of boxes used.
//
//	boxTypes := []*model.Bin{
//	    model.NewBin("small",  30, 25, 20, 5),
//	    model.NewBin("medium", 40, 35, 30, 15),
//	    model.NewBin("large",  60, 50, 40, 25),
//	}
//	items := []*model.Item{
//	    model.NewItem("item-1", 25, 20, 15, 2),
//	    model.NewItem("item-2", 35, 30, 25, 8),
//	    // ...
//	}
//	result, err := gopackx.Pack(ctx, boxTypes, items)
//
// By default, Pack uses the TrialPacking solver with lookahead (fast, good
// results). Enable [Optimize] for the metaheuristic solver that redistributes
// items across bins to minimize total box count at the cost of more computation.
//
// For fine-grained control over placement engines, strategies, and solver
// parameters, use the lower-level packages directly:
//
//   - [github.com/jcoruiz/gopackx/pkg/packer] - simple packing into pre-created bins
//   - [github.com/jcoruiz/gopackx/pkg/solver] - advanced solvers (TrialPacking, Metaheuristic, BranchBound, Parallel)
//   - [github.com/jcoruiz/gopackx/pkg/placement] - placement engines (Pivot, ExtremePoint, MaxRects, LAFF)
//   - [github.com/jcoruiz/gopackx/pkg/strategy] - item ordering and bin selection strategies
package gopackx

import (
	"context"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/solver"
)

// PackOption configures the Pack function.
type PackOption func(*packConfig)

type packConfig struct {
	optimize  bool
	newEngine func() placement.Engine
}

// Optimize enables the metaheuristic solver that redistributes items across
// bins to minimize total box count. This is slower (~30ms for 17 items) but
// can find solutions that greedy approaches miss.
//
// Without this option, Pack uses TrialPacking with lookahead which is fast
// (~100µs) and produces good results for most cases.
func Optimize() PackOption {
	return func(c *packConfig) { c.optimize = true }
}

// WithEngine sets a custom placement engine factory. By default, PivotEngine
// is used (fastest general-purpose engine).
func WithEngine(newEngine func() placement.Engine) PackOption {
	return func(c *packConfig) { c.newEngine = newEngine }
}

// Pack selects the best combination of boxes to fit all items, minimizing the
// total number of boxes used. Each box type in binTypes can be used multiple
// times (the solver creates instances as needed).
//
// Pack respects context deadlines and cancellation, returning the best solution
// found so far when the context expires.
func Pack(ctx context.Context, binTypes []*model.Bin, items []*model.Item, opts ...PackOption) (*model.Result, error) {
	cfg := &packConfig{
		newEngine: func() placement.Engine { return placement.NewPivotEngine() },
	}
	for _, opt := range opts {
		opt(cfg)
	}

	var s solver.Solver
	if cfg.optimize {
		s = solver.NewMetaheuristic(cfg.newEngine)
	} else {
		s = solver.NewTrialPacking(cfg.newEngine, solver.WithLookahead())
	}

	return s.Solve(ctx, binTypes, items)
}
