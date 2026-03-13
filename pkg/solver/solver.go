// Package solver provides advanced search algorithms for optimal packing.
package solver

import (
	"context"

	"github.com/jcoruiz/gopackx/pkg/model"
)

// Solver finds optimal or near-optimal packing solutions within a time budget.
type Solver interface {
	Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error)
}

func computeStats(bins []*model.Bin, allItems []*model.Item, unfitted []*model.Item) model.PackingStats {
	activeBins := 0
	totalVolPct := 0.0
	totalWeight := 0.0
	totalMaxWeight := 0.0

	for _, bin := range bins {
		if len(bin.Items) > 0 {
			activeBins++
			totalVolPct += bin.VolumeUsedPct()
			totalWeight += bin.TotalWeight()
			totalMaxWeight += bin.MaxWeight
		}
	}

	avgVolPct := 0.0
	avgWeightPct := 0.0
	if activeBins > 0 {
		avgVolPct = totalVolPct / float64(activeBins)
		if totalMaxWeight > 0 {
			avgWeightPct = totalWeight / totalMaxWeight * 100
		}
	}

	return model.PackingStats{
		TotalBins:     activeBins,
		TotalItems:    len(allItems),
		FittedItems:   len(allItems) - len(unfitted),
		UnfittedCount: len(unfitted),
		VolumeUsedPct: avgVolPct,
		WeightUsedPct: avgWeightPct,
	}
}
