package solver

import (
	"context"
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// Verify interface compliance.
var _ Solver = (*TrialPacking)(nil)

// TrialPacking selects which bin type to open by running the actual placement
// engine on temporary copies. For each candidate bin type it simulates a
// greedy packing of remaining items and picks the type with the best fill
// ratio, minimizing wasted space and total bins used.
//
// When lookahead is enabled (Level 4), after each trial simulation the solver
// also estimates how many additional bins will be needed for the leftover items
// and selects the bin type that minimizes total estimated bins.
type TrialPacking struct {
	newEngine func() placement.Engine
	itemSort  strategy.Type
	lookahead bool
}

// TrialOption configures the TrialPacking solver.
type TrialOption func(*TrialPacking)

// WithLookahead enables the look-ahead estimator (Level 4).
// After simulating each candidate bin, the solver estimates how many more bins
// are needed for the remaining items and minimizes total bins.
func WithLookahead() TrialOption {
	return func(tp *TrialPacking) { tp.lookahead = true }
}

// TrialItemSort sets the item ordering strategy (default BestFitDecreasing).
func TrialItemSort(st strategy.Type) TrialOption {
	return func(tp *TrialPacking) { tp.itemSort = st }
}

// NewTrialPacking creates a new TrialPacking solver.
// The engine factory creates fresh instances for each trial simulation.
func NewTrialPacking(newEngine func() placement.Engine, opts ...TrialOption) *TrialPacking {
	tp := &TrialPacking{
		newEngine: newEngine,
		itemSort:  strategy.BestFitDecreasing,
	}
	for _, opt := range opts {
		opt(tp)
	}
	return tp
}

// Solve packs items into bins using trial-based bin type selection.
// The bins parameter represents available bin types (templates); the solver
// clones them as needed to create new bin instances.
func (tp *TrialPacking) Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error) {
	if len(bins) == 0 || len(items) == 0 {
		return &model.Result{
			Bins:          bins,
			UnfittedItems: items,
			Stats:         computeStats(bins, items, items),
		}, nil
	}

	remaining := resetItems(items)
	strategy.SortItems(remaining, tp.itemSort)

	var openBins []*model.Bin
	var unfitted []*model.Item
	engine := tp.newEngine()

	for len(remaining) > 0 {
		if ctx.Err() != nil {
			unfitted = append(unfitted, remaining...)
			remaining = nil
			break
		}

		// Try to place the first unplaced item in an existing open bin.
		item := remaining[0]
		placed := false

		// Sort existing open bins by remaining volume ascending (best fit).
		candidates := sortOpenBins(openBins, item)
		for _, bin := range candidates {
			if engine.PlaceItem(bin, item) {
				placed = true
				break
			}
		}

		if placed {
			remaining = remaining[1:]
			continue
		}

		// No existing bin can fit this item. Run trial packing to decide
		// which bin type to open.
		best := tp.selectBinType(ctx, bins, remaining)
		if best.binTypeIdx < 0 {
			// Item doesn't fit in any bin type at all.
			unfitted = append(unfitted, item)
			remaining = remaining[1:]
			continue
		}

		// Open a new bin of the selected type.
		newBin := cloneBinEmpty(bins[best.binTypeIdx])
		newBin.ID = binInstanceID(bins[best.binTypeIdx].ID, len(openBins))

		// Place the current item in the new bin.
		if !engine.PlaceItem(newBin, item) {
			unfitted = append(unfitted, item)
			remaining = remaining[1:]
			continue
		}
		remaining = remaining[1:]

		openBins = append(openBins, newBin)
	}

	result := &model.Result{
		Bins:          openBins,
		UnfittedItems: unfitted,
		Stats:         computeStats(openBins, items, unfitted),
	}
	return result, ctx.Err()
}

// trialScore holds the outcome of simulating packing into a candidate bin type.
type trialScore struct {
	binTypeIdx    int
	fillRatio     float64
	fittedCount   int
	estimatedBins float64 // only used with lookahead
}

// selectBinType runs trial packing for each bin type and returns the best.
func (tp *TrialPacking) selectBinType(ctx context.Context, binTypes []*model.Bin, remaining []*model.Item) trialScore {
	best := trialScore{binTypeIdx: -1}

	for i, bt := range binTypes {
		if ctx.Err() != nil {
			break
		}

		score := tp.runTrial(bt, remaining, i)

		if score.fittedCount == 0 {
			continue
		}

		if tp.lookahead {
			score.estimatedBins = tp.estimateTotalBins(binTypes, remaining, score)
		}

		if tp.isTrialBetter(score, best) {
			best = score
		}
	}

	return best
}

// runTrial simulates packing remaining items into a fresh bin of the given type.
// The first item in remaining is the one that must be placed; if it doesn't fit,
// the trial is considered non-viable (fittedCount = 0).
func (tp *TrialPacking) runTrial(binType *model.Bin, remaining []*model.Item, typeIdx int) trialScore {
	trialBin := cloneBinEmpty(binType)
	trialItems := resetItems(remaining)
	trialEngine := tp.newEngine()

	for _, item := range trialItems {
		trialEngine.PlaceItem(trialBin, item)
	}

	// The first item is the one we need to place right now.
	// If it wasn't placed, this bin type is not viable.
	if !trialItems[0].Placed {
		return trialScore{binTypeIdx: typeIdx}
	}

	fittedCount := len(trialBin.Items)
	fillRatio := 0.0
	if trialBin.Volume > 0 && fittedCount > 0 {
		fillRatio = trialBin.UsedVolume() / trialBin.Volume
	}

	return trialScore{
		binTypeIdx:  typeIdx,
		fillRatio:   fillRatio,
		fittedCount: fittedCount,
	}
}

// estimateTotalBins estimates the total number of bins needed: 1 (this bin)
// plus a lower-bound estimate for the items that didn't fit in the trial.
func (tp *TrialPacking) estimateTotalBins(binTypes []*model.Bin, remaining []*model.Item, score trialScore) float64 {
	// Calculate remaining volume and weight after this trial bin.
	leftoverVol := 0.0
	leftoverWeight := 0.0
	leftoverCount := len(remaining) - score.fittedCount

	if leftoverCount <= 0 {
		return 1.0
	}

	// Sum the volume/weight of items that wouldn't fit.
	// Since we don't know exactly which items fit, use the smallest volumes
	// (trial packs greedily, so large items go first, small ones are leftovers).
	// But actually the simpler lower bound is total remaining - what fit.
	totalVol := 0.0
	totalWeight := 0.0
	for _, item := range remaining {
		totalVol += item.Volume
		totalWeight += item.Weight
	}

	// Volume that was packed in the trial.
	packedVol := score.fillRatio * binTypes[score.binTypeIdx].Volume
	leftoverVol = totalVol - packedVol
	leftoverWeight = totalWeight // conservative: ignore weight packed

	// Find the largest bin type for the lower bound.
	maxBinVol := 0.0
	maxBinWeight := 0.0
	for _, bt := range binTypes {
		if bt.Volume > maxBinVol {
			maxBinVol = bt.Volume
		}
		if bt.MaxWeight > maxBinWeight {
			maxBinWeight = bt.MaxWeight
		}
	}

	volBound := 0.0
	if maxBinVol > 0 {
		volBound = math.Ceil(leftoverVol / maxBinVol)
	}
	weightBound := 0.0
	if maxBinWeight > 0 {
		weightBound = math.Ceil(leftoverWeight / maxBinWeight)
	}

	futureBins := math.Max(volBound, weightBound)

	return 1.0 + futureBins
}

// isTrialBetter returns true if score a is better than score b.
func (tp *TrialPacking) isTrialBetter(a, b trialScore) bool {
	if b.binTypeIdx < 0 {
		return true
	}

	if tp.lookahead {
		// Level 4: minimize estimated total bins first.
		if a.estimatedBins != b.estimatedBins {
			return a.estimatedBins < b.estimatedBins
		}
		// Tie-break: higher fill ratio.
		if a.fillRatio != b.fillRatio {
			return a.fillRatio > b.fillRatio
		}
		// Tie-break: prefer smaller bin (less waste).
		return a.binTypeIdx < b.binTypeIdx
	}

	// Level 3: maximize fill ratio.
	if a.fillRatio != b.fillRatio {
		return a.fillRatio > b.fillRatio
	}
	// Tie-break: more items fitted.
	if a.fittedCount != b.fittedCount {
		return a.fittedCount > b.fittedCount
	}
	// Tie-break: prefer smaller bin type.
	return a.binTypeIdx < b.binTypeIdx
}

// sortOpenBins returns existing open bins sorted by remaining volume ascending
// (best fit), filtered by weight capacity.
func sortOpenBins(bins []*model.Bin, item *model.Item) []*model.Bin {
	return strategy.SortBinsForItem(bins, item, strategy.BestFitDecreasing)
}

// binInstanceID generates a unique bin ID for a new instance.
func binInstanceID(typeID string, seq int) string {
	return typeID + "-" + itoa(seq)
}

// itoa converts an int to a string without importing strconv.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
