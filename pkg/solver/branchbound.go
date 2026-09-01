package solver

import (
	"context"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Solver = (*BranchBound)(nil)

// BranchBound exhaustively searches for the item ordering that maximizes
// bin utilization. It processes bins sequentially, optimizing each one.
//
// Fast variant (default): iterates over item permutations, packing greedily
// per permutation. Practical for ≤12 items without deadline.
//
// Full variant: DFS with per-item rotation selection. More thorough but slower.
// Practical for ≤8 items without deadline.
//
// Both variants respect context deadline and return the best solution found.
type BranchBound struct {
	newEngine func() placement.Engine
	full      bool
}

// BBOption configures the BranchBound solver.
type BBOption func(*BranchBound)

// BBFull enables the full variant that tries all rotation combinations per item.
func BBFull() BBOption {
	return func(bb *BranchBound) { bb.full = true }
}

// NewBranchBound creates a new BranchBound solver.
// The engine factory creates fresh engine instances for each search branch.
func NewBranchBound(newEngine func() placement.Engine, opts ...BBOption) *BranchBound {
	bb := &BranchBound{newEngine: newEngine}
	for _, opt := range opts {
		opt(bb)
	}
	return bb
}

// Solve processes bins sequentially, using B&B to maximize items in each bin.
func (bb *BranchBound) Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error) {
	if len(bins) == 0 || len(items) == 0 {
		return &model.Result{
			Bins:          bins,
			UnfittedItems: items,
			Stats:         computeStats(bins, items, items),
		}, nil
	}

	remaining := resetItems(items)
	resultBins := make([]*model.Bin, 0, len(bins))

	for _, bin := range bins {
		if ctx.Err() != nil || len(remaining) == 0 {
			break
		}

		best := bb.optimizeSingleBin(ctx, bin, remaining)
		resultBins = append(resultBins, best.bin)
		remaining = best.unfitted
	}

	return &model.Result{
		Bins:          resultBins,
		UnfittedItems: remaining,
		Stats:         computeStats(resultBins, items, remaining),
	}, nil
}

type singleBinResult struct {
	bin      *model.Bin
	unfitted []*model.Item
	count    int
}

func (bb *BranchBound) optimizeSingleBin(ctx context.Context, origBin *model.Bin, items []*model.Item) singleBinResult {
	if bb.full {
		return bb.optimizeFull(ctx, origBin, items)
	}
	return bb.optimizeFast(ctx, origBin, items)
}

// optimizeFast tries all permutations of items, packing each greedily.
func (bb *BranchBound) optimizeFast(ctx context.Context, origBin *model.Bin, items []*model.Item) singleBinResult {
	n := len(items)
	engine := bb.newEngine()

	// Greedy seed: pack in current order.
	seedBin := cloneBinEmpty(origBin)
	seedItems := resetItems(items)
	for _, item := range seedItems {
		engine.PlaceItem(seedBin, item)
	}

	best := singleBinResult{
		bin:      seedBin,
		count:    len(seedBin.Items),
		unfitted: collectUnfitted(seedItems),
	}

	if best.count >= n || n <= 1 {
		return best
	}

	// Try all permutations via next-permutation algorithm.
	indices := make([]int, n)
	for i := range indices {
		indices[i] = i
	}

	for nextPermutation(indices) {
		if ctx.Err() != nil {
			break
		}

		binCopy := cloneBinEmpty(origBin)
		itemsCopy := resetItems(items)
		for _, idx := range indices {
			engine.PlaceItem(binCopy, itemsCopy[idx])
		}

		placed := len(binCopy.Items)
		if placed > best.count {
			best.bin = binCopy
			best.count = placed
			best.unfitted = collectUnfitted(itemsCopy)
			if placed >= n {
				break
			}
		}
	}

	return best
}

// optimizeFull uses DFS with rotation iteration for exhaustive search.
func (bb *BranchBound) optimizeFull(ctx context.Context, origBin *model.Bin, items []*model.Item) singleBinResult {
	n := len(items)
	engine := bb.newEngine()

	// Greedy seed.
	seedBin := cloneBinEmpty(origBin)
	seedItems := resetItems(items)
	for _, item := range seedItems {
		engine.PlaceItem(seedBin, item)
	}

	best := singleBinResult{
		bin:      seedBin,
		count:    len(seedBin.Items),
		unfitted: collectUnfitted(seedItems),
	}

	if best.count >= n || n <= 1 {
		return best
	}

	// DFS with backtracking.
	used := make([]bool, n)
	emptyBin := cloneBinEmpty(origBin)
	bb.dfsFull(ctx, engine, emptyBin, items, used, 0, &best)

	return best
}

func (bb *BranchBound) dfsFull(
	ctx context.Context,
	engine placement.Engine,
	parentBin *model.Bin,
	items []*model.Item,
	used []bool,
	depth int,
	best *singleBinResult,
) {
	if ctx.Err() != nil {
		return
	}

	// Pruning: can't beat best even placing all remaining items.
	remaining := 0
	for _, u := range used {
		if !u {
			remaining++
		}
	}
	if depth+remaining <= best.count {
		return
	}

	for i := range items {
		if used[i] {
			continue
		}

		for _, rt := range rotation.AllowedFor(items[i]) {
			binCopy := snapshotBin(parentBin)
			itemCopy := resetItem(items[i])
			itemCopy.AllowedRotations = []model.RotationType{rt}

			if !engine.PlaceItem(binCopy, itemCopy) {
				continue
			}

			used[i] = true
			newDepth := depth + 1

			if newDepth > best.count {
				best.count = newDepth
				best.bin = snapshotBin(binCopy)
				best.unfitted = collectUnfittedByUsed(items, used)
			}

			if newDepth < len(items) {
				bb.dfsFull(ctx, engine, binCopy, items, used, newDepth, best)
			}

			used[i] = false

			if best.count >= len(items) {
				return // optimal found
			}
		}
	}
}

func collectUnfitted(items []*model.Item) []*model.Item {
	var out []*model.Item
	for _, item := range items {
		if !item.Placed {
			out = append(out, item)
		}
	}
	return out
}

func collectUnfittedByUsed(items []*model.Item, used []bool) []*model.Item {
	var out []*model.Item
	for i, item := range items {
		if !used[i] {
			out = append(out, resetItem(item))
		}
	}
	return out
}

// nextPermutation generates the next lexicographic permutation in-place.
// Returns false when all permutations have been generated.
func nextPermutation(a []int) bool {
	n := len(a)
	if n <= 1 {
		return false
	}

	// Find largest i such that a[i] < a[i+1].
	i := n - 2
	for i >= 0 && a[i] >= a[i+1] {
		i--
	}
	if i < 0 {
		return false
	}

	// Find largest j such that a[i] < a[j].
	j := n - 1
	for a[j] <= a[i] {
		j--
	}

	a[i], a[j] = a[j], a[i]

	// Reverse a[i+1:].
	for l, r := i+1, n-1; l < r; l, r = l+1, r-1 {
		a[l], a[r] = a[r], a[l]
	}

	return true
}
