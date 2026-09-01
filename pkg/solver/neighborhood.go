package solver

import (
	"math/rand"
	"sort"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// solution is an abstract item-to-bin assignment used by the metaheuristic.
// Neighborhood operators modify this assignment; 3D feasibility is verified
// separately via repackBin.
type solution struct {
	assignments []int // assignments[i] = bin index for item i (-1 = unfitted)
	binTypeIdx  []int // binTypeIdx[b] = index into binTypes for bin b
	nBins       int
	score       solutionScore
}

type solutionScore struct {
	totalBins   int
	unfitted    int
	avgFillPct  float64
	totalVolPct float64
	totalCost   float64
}

type neighborhoodOp int

const (
	opMove neighborhoodOp = iota
	opSwap
	opRepack
	opChangeType
)

// cloneSolution creates a deep copy of a solution.
func cloneSolution(s *solution) *solution {
	c := &solution{
		assignments: make([]int, len(s.assignments)),
		binTypeIdx:  make([]int, len(s.binTypeIdx)),
		nBins:       s.nBins,
		score:       s.score,
	}
	copy(c.assignments, s.assignments)
	copy(c.binTypeIdx, s.binTypeIdx)
	return c
}

// extractSolution converts a Result (from seed solver) into a solution.
// binTypes is used to map each result bin to its type index.
func extractSolution(result *model.Result, items []*model.Item, binTypes []*model.Bin) *solution {
	sol := &solution{
		assignments: make([]int, len(items)),
	}

	// Initialize all as unfitted.
	for i := range sol.assignments {
		sol.assignments[i] = -1
	}

	// Build a mapping from item ID to index in the original items slice.
	itemIdx := make(map[string][]int, len(items))
	for i, item := range items {
		itemIdx[item.ID] = append(itemIdx[item.ID], i)
	}

	// Track which indices have been assigned (for duplicate IDs).
	assigned := make([]bool, len(items))

	for _, bin := range result.Bins {
		if len(bin.Items) == 0 {
			continue
		}
		binIdx := sol.nBins
		sol.nBins++

		// Find matching bin type.
		typeIdx := matchBinType(bin, binTypes)
		sol.binTypeIdx = append(sol.binTypeIdx, typeIdx)

		for _, placedItem := range bin.Items {
			// Find an unassigned item index with this ID.
			for _, idx := range itemIdx[placedItem.ID] {
				if !assigned[idx] {
					sol.assignments[idx] = binIdx
					assigned[idx] = true
					break
				}
			}
		}
	}

	sol.score = scoreSolution(sol, items, binTypes)
	return sol
}

// matchBinType finds which bin type matches the given bin by dimensions.
func matchBinType(bin *model.Bin, binTypes []*model.Bin) int {
	for i, bt := range binTypes {
		if bt.Width == bin.Width && bt.Height == bin.Height &&
			bt.Depth == bin.Depth && bt.MaxWeight == bin.MaxWeight {
			return i
		}
	}
	return 0
}

// scoreSolution computes the score of a solution.
func scoreSolution(sol *solution, items []*model.Item, binTypes []*model.Bin) solutionScore {
	unfitted := 0
	for _, a := range sol.assignments {
		if a < 0 {
			unfitted++
		}
	}

	// Compute per-bin volume usage.
	binVols := make([]float64, sol.nBins)
	for i, a := range sol.assignments {
		if a >= 0 {
			binVols[a] += items[i].Volume
		}
	}

	totalPct := 0.0
	totalCost := 0.0
	activeBins := 0
	for b := range sol.nBins {
		if binVols[b] > 0 {
			activeBins++
			totalPct += binVols[b] / binTypes[sol.binTypeIdx[b]].Volume * 100
			totalCost += binTypes[sol.binTypeIdx[b]].Cost
		}
	}

	avgPct := 0.0
	if activeBins > 0 {
		avgPct = totalPct / float64(activeBins)
	}

	return solutionScore{
		totalBins:   activeBins,
		unfitted:    unfitted,
		avgFillPct:  avgPct,
		totalVolPct: totalPct,
		totalCost:   totalCost,
	}
}

// isBetterSol returns true if a is better than b.
func isBetterSol(a, b *solution) bool {
	// Fewer unfitted items first.
	if a.score.unfitted != b.score.unfitted {
		return a.score.unfitted < b.score.unfitted
	}
	// When costs are set, minimize total cost.
	if a.score.totalCost > 0 || b.score.totalCost > 0 {
		if a.score.totalCost != b.score.totalCost {
			return a.score.totalCost < b.score.totalCost
		}
	}
	// Fewer bins.
	if a.score.totalBins != b.score.totalBins {
		return a.score.totalBins < b.score.totalBins
	}
	// Higher average fill.
	return a.score.avgFillPct > b.score.avgFillPct
}

// --- Neighborhood operators ---

// itemsInBin returns indices of items assigned to bin b.
func itemsInBin(sol *solution, b int) []int {
	var out []int
	for i, a := range sol.assignments {
		if a == b {
			out = append(out, i)
		}
	}
	return out
}

// binFillVolume returns the total item volume assigned to bin b.
func binFillVolume(sol *solution, b int, items []*model.Item) float64 {
	vol := 0.0
	for i, a := range sol.assignments {
		if a == b {
			vol += items[i].Volume
		}
	}
	return vol
}

// binFillWeight returns the total item weight assigned to bin b.
func binFillWeight(sol *solution, b int, items []*model.Item) float64 {
	w := 0.0
	for i, a := range sol.assignments {
		if a == b {
			w += items[i].Weight
		}
	}
	return w
}

// shakeMove tries to move an item from the least-filled bin to another.
func shakeMove(sol *solution, items []*model.Item, binTypes []*model.Bin) *solution {
	if sol.nBins < 2 {
		return nil
	}

	// Find the bin with minimum fill ratio.
	srcBin := -1
	minFill := 2.0
	for b := range sol.nBins {
		vol := binFillVolume(sol, b, items)
		if vol == 0 {
			continue
		}
		fill := vol / binTypes[sol.binTypeIdx[b]].Volume
		if fill < minFill {
			minFill = fill
			srcBin = b
		}
	}
	if srcBin < 0 {
		return nil
	}

	srcItems := itemsInBin(sol, srcBin)
	if len(srcItems) == 0 {
		return nil
	}

	// Sort source items by volume ascending (move smallest first).
	sort.Slice(srcItems, func(i, j int) bool {
		return items[srcItems[i]].Volume < items[srcItems[j]].Volume
	})

	// Try each item against each other bin.
	for _, itemIdx := range srcItems {
		item := items[itemIdx]
		for b := range sol.nBins {
			if b == srcBin {
				continue
			}
			bt := binTypes[sol.binTypeIdx[b]]
			// Quick volume check.
			if binFillVolume(sol, b, items)+item.Volume > bt.Volume {
				continue
			}
			// Quick weight check.
			if binFillWeight(sol, b, items)+item.Weight > bt.MaxWeight {
				continue
			}

			newSol := cloneSolution(sol)
			newSol.assignments[itemIdx] = b

			// If source bin is now empty, remove it.
			if len(srcItems) == 1 {
				newSol.removeBin(srcBin)
			}
			return newSol
		}
	}
	return nil
}

// shakeSwap swaps two items between different bins.
func shakeSwap(sol *solution, items []*model.Item, binTypes []*model.Bin) *solution {
	if sol.nBins < 2 {
		return nil
	}

	// Collect all (item, bin) pairs.
	type itemBin struct {
		itemIdx int
		binIdx  int
	}
	var pairs []itemBin
	for i, a := range sol.assignments {
		if a >= 0 {
			pairs = append(pairs, itemBin{i, a})
		}
	}
	if len(pairs) < 2 {
		return nil
	}

	// Shuffle and try pairs from different bins.
	rand.Shuffle(len(pairs), func(i, j int) {
		pairs[i], pairs[j] = pairs[j], pairs[i]
	})

	for x := range len(pairs) - 1 {
		for y := x + 1; y < len(pairs); y++ {
			p1, p2 := pairs[x], pairs[y]
			if p1.binIdx == p2.binIdx {
				continue
			}

			i1, i2 := items[p1.itemIdx], items[p2.itemIdx]
			bt1, bt2 := binTypes[sol.binTypeIdx[p1.binIdx]], binTypes[sol.binTypeIdx[p2.binIdx]]

			// Weight check after swap.
			w1 := binFillWeight(sol, p1.binIdx, items) - i1.Weight + i2.Weight
			w2 := binFillWeight(sol, p2.binIdx, items) - i2.Weight + i1.Weight
			if w1 > bt1.MaxWeight || w2 > bt2.MaxWeight {
				continue
			}

			// Volume check after swap.
			v1 := binFillVolume(sol, p1.binIdx, items) - i1.Volume + i2.Volume
			v2 := binFillVolume(sol, p2.binIdx, items) - i2.Volume + i1.Volume
			if v1 > bt1.Volume || v2 > bt2.Volume {
				continue
			}

			newSol := cloneSolution(sol)
			newSol.assignments[p1.itemIdx] = p2.binIdx
			newSol.assignments[p2.itemIdx] = p1.binIdx
			return newSol
		}
		// Limit search to avoid O(n^2) on large inputs.
		if x > 50 {
			break
		}
	}
	return nil
}

// shakeRepack tries to eliminate the least-filled bin by redistributing its items.
// It uses the actual placement engine to verify each redistribution is 3D-feasible.
func shakeRepack(sol *solution, items []*model.Item, binTypes []*model.Bin) *solution {
	return shakeRepackWithEngine(sol, items, binTypes, nil)
}

// shakeRepackWithEngine tries to eliminate a bin using optional engine validation.
func shakeRepackWithEngine(sol *solution, items []*model.Item, binTypes []*model.Bin, newEngine func() placement.Engine) *solution {
	if sol.nBins < 2 {
		return nil
	}

	// Rank bins by fill ratio ascending (try to eliminate the emptiest first).
	type binFill struct {
		idx  int
		fill float64
	}
	var ranked []binFill
	for b := range sol.nBins {
		vol := binFillVolume(sol, b, items)
		if vol == 0 {
			continue
		}
		fill := vol / binTypes[sol.binTypeIdx[b]].Volume
		ranked = append(ranked, binFill{b, fill})
	}
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].fill < ranked[j].fill
	})

	// Try to eliminate each candidate bin (starting with the emptiest).
	for _, candidate := range ranked {
		targetBin := candidate.idx
		targetItems := itemsInBin(sol, targetBin)
		if len(targetItems) == 0 {
			continue
		}

		// Try multiple redistribution orders.
		result := tryRedistribute(sol, targetBin, targetItems, items, binTypes, newEngine)
		if result != nil {
			return result
		}
	}
	return nil
}

// tryRedistribute attempts to redistribute items from targetBin to other bins.
// Tries multiple item orderings and bin orderings to maximize chances.
func tryRedistribute(sol *solution, targetBin int, targetItems []int, items []*model.Item, binTypes []*model.Bin, newEngine func() placement.Engine) *solution {
	// Try different item orderings: volume desc, volume asc, and shuffled.
	orderings := make([][]int, 3)
	for o := range 3 {
		ordering := make([]int, len(targetItems))
		copy(ordering, targetItems)
		switch o {
		case 0: // volume descending
			sort.Slice(ordering, func(i, j int) bool {
				return items[ordering[i]].Volume > items[ordering[j]].Volume
			})
		case 1: // volume ascending
			sort.Slice(ordering, func(i, j int) bool {
				return items[ordering[i]].Volume < items[ordering[j]].Volume
			})
		case 2: // shuffled
			rand.Shuffle(len(ordering), func(i, j int) {
				ordering[i], ordering[j] = ordering[j], ordering[i]
			})
		}
		orderings[o] = ordering
	}

	for _, ordering := range orderings {
		newSol := cloneSolution(sol)
		newSol.removeBin(targetBin)

		// Build destination bins with remapped indices (after removal).
		destBins := make([]int, 0, newSol.nBins)
		for b := range newSol.nBins {
			destBins = append(destBins, b)
		}

		allPlaced := true
		for _, itemIdx := range ordering {
			item := items[itemIdx]
			placed := false

			// Sort destinations by remaining space ascending (best fit).
			sortedDest := make([]int, len(destBins))
			copy(sortedDest, destBins)
			sort.Slice(sortedDest, func(i, j int) bool {
				di, dj := sortedDest[i], sortedDest[j]
				ri := binTypes[newSol.binTypeIdx[di]].Volume - binFillVolume(newSol, di, items)
				rj := binTypes[newSol.binTypeIdx[dj]].Volume - binFillVolume(newSol, dj, items)
				return ri < rj
			})

			for _, b := range sortedDest {
				bt := binTypes[newSol.binTypeIdx[b]]
				if binFillVolume(newSol, b, items)+item.Volume > bt.Volume {
					continue
				}
				if binFillWeight(newSol, b, items)+item.Weight > bt.MaxWeight {
					continue
				}

				// If we have an engine, verify 3D feasibility immediately.
				if newEngine != nil {
					testItems := make([]*model.Item, 0)
					for ii, a := range newSol.assignments {
						if a == b {
							testItems = append(testItems, items[ii])
						}
					}
					testItems = append(testItems, item)
					if _, ok := repackBin(newEngine, bt, testItems); !ok {
						continue
					}
				}

				newSol.assignments[itemIdx] = b
				placed = true
				break
			}
			if !placed {
				allPlaced = false
				break
			}
		}

		if allPlaced {
			return newSol
		}
	}

	return nil
}

// shakeChangeType tries to downsize a bin to a smaller type.
func shakeChangeType(sol *solution, items []*model.Item, binTypes []*model.Bin) *solution {
	// Try each bin, attempt to use a smaller bin type.
	binOrder := make([]int, sol.nBins)
	for i := range binOrder {
		binOrder[i] = i
	}
	rand.Shuffle(len(binOrder), func(i, j int) {
		binOrder[i], binOrder[j] = binOrder[j], binOrder[i]
	})

	for _, b := range binOrder {
		currentTypeIdx := sol.binTypeIdx[b]
		currentVol := binTypes[currentTypeIdx].Volume
		bItems := itemsInBin(sol, b)
		if len(bItems) == 0 {
			continue
		}

		// Try smaller bin types, sorted by volume descending (try the biggest-smaller first).
		type candidate struct {
			idx int
			vol float64
		}
		var candidates []candidate
		for i, bt := range binTypes {
			if bt.Volume < currentVol {
				candidates = append(candidates, candidate{i, bt.Volume})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].vol > candidates[j].vol
		})

		for _, c := range candidates {
			bt := binTypes[c.idx]
			// Quick checks.
			fillVol := binFillVolume(sol, b, items)
			fillWeight := binFillWeight(sol, b, items)
			if fillVol > bt.Volume || fillWeight > bt.MaxWeight {
				continue
			}
			// Check that each item's smallest dimension fits.
			fits := true
			for _, itemIdx := range bItems {
				item := items[itemIdx]
				dims := sortedDims(item)
				binDims := sortedDims3(bt.Width, bt.Height, bt.Depth)
				if dims[0] > binDims[0] || dims[1] > binDims[1] || dims[2] > binDims[2] {
					fits = false
					break
				}
			}
			if !fits {
				continue
			}

			newSol := cloneSolution(sol)
			newSol.binTypeIdx[b] = c.idx
			return newSol
		}
	}
	return nil
}

// removeBin removes bin b from the solution, remapping all bin indices.
func (s *solution) removeBin(b int) {
	// Remap assignments: bins after b shift down by 1.
	for i, a := range s.assignments {
		if a == b {
			s.assignments[i] = -1
		} else if a > b {
			s.assignments[i] = a - 1
		}
	}
	// Remove from binTypeIdx.
	s.binTypeIdx = append(s.binTypeIdx[:b], s.binTypeIdx[b+1:]...)
	s.nBins--
}

// sortedDims returns [min, mid, max] dimensions of an item.
func sortedDims(item *model.Item) [3]float64 {
	d := [3]float64{item.Width, item.Height, item.Depth}
	if d[0] > d[1] {
		d[0], d[1] = d[1], d[0]
	}
	if d[1] > d[2] {
		d[1], d[2] = d[2], d[1]
	}
	if d[0] > d[1] {
		d[0], d[1] = d[1], d[0]
	}
	return d
}

// sortedDims3 returns [min, mid, max] of three floats.
func sortedDims3(a, b, c float64) [3]float64 {
	d := [3]float64{a, b, c}
	if d[0] > d[1] {
		d[0], d[1] = d[1], d[0]
	}
	if d[1] > d[2] {
		d[1], d[2] = d[2], d[1]
	}
	if d[0] > d[1] {
		d[0], d[1] = d[1], d[0]
	}
	return d
}

// repackBin verifies that a set of items physically fits in a bin type
// by running the actual placement engine. Returns the packed bin if all fit.
func repackBin(newEngine func() placement.Engine, binType *model.Bin, items []*model.Item) (*model.Bin, bool) {
	bin := cloneBinEmpty(binType)
	copies := resetItems(items)

	// Try BestFitDecreasing order.
	strategy.SortItems(copies, strategy.BestFitDecreasing)

	engine := newEngine()
	for _, item := range copies {
		engine.PlaceItem(bin, item)
	}

	if len(bin.Items) == len(items) {
		return bin, true
	}

	// Try reverse order as fallback.
	bin2 := cloneBinEmpty(binType)
	copies2 := resetItems(items)
	strategy.SortItems(copies2, strategy.Greedy) // volume ascending

	engine2 := newEngine()
	for _, item := range copies2 {
		engine2.PlaceItem(bin2, item)
	}

	if len(bin2.Items) == len(items) {
		return bin2, true
	}

	// Try MinimizeBins order.
	bin3 := cloneBinEmpty(binType)
	copies3 := resetItems(items)
	strategy.SortItems(copies3, strategy.MinimizeBins)

	engine3 := newEngine()
	for _, item := range copies3 {
		engine3.PlaceItem(bin3, item)
	}

	if len(bin3.Items) == len(items) {
		return bin3, true
	}

	return nil, false
}

// revalidate checks that all bins in the solution are 3D-feasible.
// Only checks bins listed in dirtyBins. Returns true if all pass.
func revalidate(newEngine func() placement.Engine, sol *solution, items []*model.Item, binTypes []*model.Bin, dirtyBins []int) bool {
	for _, b := range dirtyBins {
		if b >= sol.nBins {
			continue
		}
		bItems := itemsInBin(sol, b)
		if len(bItems) == 0 {
			continue
		}
		binItems := make([]*model.Item, len(bItems))
		for i, idx := range bItems {
			binItems[i] = items[idx]
		}
		_, ok := repackBin(newEngine, binTypes[sol.binTypeIdx[b]], binItems)
		if !ok {
			return false
		}
	}
	return true
}
