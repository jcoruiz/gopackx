// Package solver provides advanced search algorithms for optimal packing.
package solver

import (
	"context"

	"github.com/jcoruiz/gopackx/internal/stats"
	"github.com/jcoruiz/gopackx/pkg/model"
)

// Solver finds optimal or near-optimal packing solutions within a time budget.
//
// Invalid input (see [model.Validate]) makes Solve return a nil result and
// an error that wraps [model.ErrInvalidInput].
type Solver interface {
	Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error)
}

func computeStats(bins []*model.Bin, allItems, unfitted []*model.Item) model.PackingStats {
	return stats.Compute(bins, allItems, unfitted)
}
