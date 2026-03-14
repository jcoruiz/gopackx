// Package strategy provides bin selection and item ordering strategies.
package strategy

import (
	"math"
	"sort"

	"github.com/jcoruiz/gopackx/pkg/model"
)

// Type represents a packing strategy.
type Type int

const (
	MinimizeBins     Type = iota // Vol desc, First Fit
	Greedy                       // Vol asc, First Fit
	BestFit                      // Vol asc, Best Fit (smallest remaining)
	BestFitDecreasing            // Vol desc, Best Fit
	NextFit                      // Vol asc, current bin only
	WorstFit                     // Vol asc, Worst Fit (largest remaining)
	AlmostWorstFit               // Vol asc, Worst Fit skipping >80% empty
)

// SortItems sorts items according to the strategy's multi-criteria ordering:
// 1. Volume (direction depends on strategy)
// 2. Load-bearing desc (sturdy items first)
// 3. Priority asc (1=highest)
// 4. Group (keep binding groups together)
func SortItems(items []*model.Item, st Type) {
	volDesc := st == MinimizeBins || st == BestFitDecreasing

	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]

		if a.Volume != b.Volume {
			if volDesc {
				return a.Volume > b.Volume
			}
			return a.Volume < b.Volume
		}
		if a.LoadBear != b.LoadBear {
			return a.LoadBear > b.LoadBear
		}
		if a.Priority != b.Priority {
			return a.Priority < b.Priority
		}
		return a.Group < b.Group
	})
}

// SortBinsForItem returns bins ordered by the strategy's selection criteria,
// filtered to those with enough remaining weight capacity.
// NextFit is not handled here; the packer manages its stateful logic.
func SortBinsForItem(bins []*model.Bin, item *model.Item, st Type) []*model.Bin {
	candidates := make([]*model.Bin, 0, len(bins))
	for _, bin := range bins {
		if bin.RemainingWeight() >= item.Weight {
			candidates = append(candidates, bin)
		}
	}
	if len(candidates) == 0 {
		return nil
	}

	switch st {
	case BestFit, BestFitDecreasing:
		sort.SliceStable(candidates, func(i, j int) bool {
			ri := candidates[i].Volume - candidates[i].UsedVolume()
			rj := candidates[j].Volume - candidates[j].UsedVolume()
			return ri < rj
		})

	case WorstFit:
		sort.SliceStable(candidates, func(i, j int) bool {
			ri := candidates[i].Volume - candidates[i].UsedVolume()
			rj := candidates[j].Volume - candidates[j].UsedVolume()
			return ri > rj
		})

	case AlmostWorstFit:
		var filtered []*model.Bin
		for _, b := range candidates {
			if len(b.Items) == 0 || b.VolumeUsedPct() >= 20 {
				filtered = append(filtered, b)
			}
		}
		if len(filtered) == 0 {
			filtered = candidates
		}
		sort.SliceStable(filtered, func(i, j int) bool {
			ri := filtered[i].Volume - filtered[i].UsedVolume()
			rj := filtered[j].Volume - filtered[j].UsedVolume()
			return ri > rj
		})
		candidates = filtered

	default:
		// MinimizeBins, Greedy, NextFit: First Fit (keep original order)
	}

	return candidates
}

// RemainingVolume returns the free volume in a bin.
func RemainingVolume(bin *model.Bin) float64 {
	r := bin.Volume - bin.UsedVolume()
	return math.Max(0, r)
}
