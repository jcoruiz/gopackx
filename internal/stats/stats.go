// Package stats computes the summary of a packing result.
package stats

import "github.com/jcoruiz/gopackx/pkg/model"

// Compute summarizes a packing. Bins without items are not counted.
// WeightUsedPct only covers bins that have a weight limit.
func Compute(bins []*model.Bin, allItems, unfitted []*model.Item) model.PackingStats {
	activeBins := 0
	totalVolPct := 0.0
	limitedWeight := 0.0
	totalMaxWeight := 0.0
	totalCost := 0.0

	for _, bin := range bins {
		if len(bin.Items) == 0 {
			continue
		}
		activeBins++
		totalVolPct += bin.VolumeUsedPct()
		totalCost += bin.Cost
		if bin.HasWeightLimit() {
			limitedWeight += bin.TotalWeight()
			totalMaxWeight += bin.MaxWeight
		}
	}

	s := model.PackingStats{
		TotalBins:     activeBins,
		TotalItems:    len(allItems),
		FittedItems:   len(allItems) - len(unfitted),
		UnfittedCount: len(unfitted),
		TotalCost:     totalCost,
	}
	if activeBins > 0 {
		s.VolumeUsedPct = totalVolPct / float64(activeBins)
	}
	if totalMaxWeight > 0 {
		s.WeightUsedPct = limitedWeight / totalMaxWeight * 100
	}
	return s
}
