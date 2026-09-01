package solver

import (
	"context"
	"strconv"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
)

// Verify interface compliance.
var _ Solver = (*Metaheuristic)(nil)

// Metaheuristic uses Variable Neighborhood Search (VNS) to optimize bin packing
// across multiple bins. Unlike greedy solvers, it can redistribute items between
// bins to reduce the total number of bins used.
//
// It starts from a seed solution (default: TrialPacking with lookahead) and
// iteratively applies neighborhood operators (MOVE, SWAP, REPACK, CHANGE_TYPE)
// to improve the solution. Each modification is validated using the actual 3D
// placement engine to ensure physical feasibility.
type Metaheuristic struct {
	newEngine    func() placement.Engine
	seedSolver   Solver
	maxIter      int
	maxNoImprove int
}

// MetaOption configures the Metaheuristic solver.
type MetaOption func(*Metaheuristic)

// MetaSeed sets the solver used to generate the initial solution.
// Default: TrialPacking with lookahead (Level 4).
func MetaSeed(s Solver) MetaOption {
	return func(m *Metaheuristic) { m.seedSolver = s }
}

// MetaMaxIter sets the maximum number of VNS iterations.
// Default: 1000.
func MetaMaxIter(n int) MetaOption {
	return func(m *Metaheuristic) { m.maxIter = n }
}

// MetaMaxNoImprove sets how many iterations without improvement before stopping.
// Default: 200.
func MetaMaxNoImprove(n int) MetaOption {
	return func(m *Metaheuristic) { m.maxNoImprove = n }
}

// NewMetaheuristic creates a new Metaheuristic solver.
func NewMetaheuristic(newEngine func() placement.Engine, opts ...MetaOption) *Metaheuristic {
	m := &Metaheuristic{
		newEngine:    newEngine,
		maxIter:      1000,
		maxNoImprove: 200,
	}
	for _, opt := range opts {
		opt(m)
	}
	if m.seedSolver == nil {
		m.seedSolver = NewTrialPacking(newEngine, WithLookahead())
	}
	return m
}

// Solve finds a near-optimal packing by starting from a seed solution and
// iteratively improving it with VNS neighborhood operators.
func (m *Metaheuristic) Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error) {
	if len(bins) == 0 || len(items) == 0 {
		return &model.Result{
			Bins:          bins,
			UnfittedItems: items,
			Stats:         computeStats(bins, items, items),
		}, nil
	}

	// Generate seed solution.
	seedResult, err := m.seedSolver.Solve(ctx, bins, items)
	if err != nil {
		return seedResult, err
	}
	if ctx.Err() != nil {
		return seedResult, nil
	}

	// Convert to abstract solution.
	current := extractSolution(seedResult, items, bins)
	best := cloneSolution(current)

	ops := []neighborhoodOp{opMove, opSwap, opRepack, opChangeType}
	noImprove := 0

	for iter := 0; iter < m.maxIter && noImprove < m.maxNoImprove; iter++ {
		if ctx.Err() != nil {
			break
		}

		improved := false
		k := 0

		for k < len(ops) {
			if ctx.Err() != nil {
				break
			}

			// Shake: generate a neighbor in neighborhood k.
			candidate := m.shake(current, ops[k], items, bins)
			if candidate == nil {
				k++
				continue
			}

			// Determine which bins changed.
			dirty := m.dirtyBins(current, candidate)

			// Revalidate changed bins with the real 3D engine.
			if !revalidate(m.newEngine, candidate, items, bins, dirty) {
				k++
				continue
			}

			// Score the candidate.
			candidate.score = scoreSolution(candidate, items, bins)

			// Accept if better.
			if isBetterSol(candidate, current) {
				current = candidate
				improved = true

				if isBetterSol(candidate, best) {
					best = cloneSolution(candidate)
					noImprove = 0
				}

				break // restart from lightest neighborhood (VNS); outer loop resets k
			}
			k++
		}

		if !improved {
			noImprove++
		}
	}

	// Materialize the best solution into a Result.
	return m.materialize(best, items, bins), nil
}

// shake applies a neighborhood operator to generate a candidate solution.
func (m *Metaheuristic) shake(sol *solution, op neighborhoodOp, items []*model.Item, binTypes []*model.Bin) *solution {
	switch op {
	case opMove:
		return shakeMove(sol, items, binTypes)
	case opSwap:
		return shakeSwap(sol, items, binTypes)
	case opRepack:
		// Use engine-validated repack for 3D feasibility during redistribution.
		return shakeRepackWithEngine(sol, items, binTypes, m.newEngine)
	case opChangeType:
		return shakeChangeType(sol, items, binTypes)
	}
	return nil
}

// dirtyBins returns bin indices that differ between two solutions.
func (m *Metaheuristic) dirtyBins(prev, cur *solution) []int {
	seen := make(map[int]struct{})
	for i := range prev.assignments {
		if prev.assignments[i] != cur.assignments[i] {
			if cur.assignments[i] >= 0 {
				seen[cur.assignments[i]] = struct{}{}
			}
		}
	}
	// Also check bins with changed types.
	for b := 0; b < cur.nBins && b < len(prev.binTypeIdx) && b < len(cur.binTypeIdx); b++ {
		if prev.binTypeIdx[b] != cur.binTypeIdx[b] {
			seen[b] = struct{}{}
		}
	}
	out := make([]int, 0, len(seen))
	for b := range seen {
		out = append(out, b)
	}
	return out
}

// materialize converts an abstract solution back into a model.Result
// by running the placement engine on each bin.
func (m *Metaheuristic) materialize(sol *solution, items []*model.Item, binTypes []*model.Bin) *model.Result {
	var resultBins []*model.Bin
	var unfitted []*model.Item

	for b := range sol.nBins {
		bItemIdxs := itemsInBin(sol, b)
		if len(bItemIdxs) == 0 {
			continue
		}

		binItems := make([]*model.Item, len(bItemIdxs))
		for i, idx := range bItemIdxs {
			binItems[i] = items[idx]
		}

		bt := binTypes[sol.binTypeIdx[b]]
		packed, ok := repackBin(m.newEngine, bt, binItems)
		if ok {
			packed.ID = bt.ID + "-" + strconv.Itoa(len(resultBins))
			resultBins = append(resultBins, packed)
		} else {
			// Shouldn't happen if revalidation passed, but handle gracefully.
			for _, item := range binItems {
				unfitted = append(unfitted, resetItem(item))
			}
		}
	}

	// Add items that were never assigned.
	for i, a := range sol.assignments {
		if a < 0 {
			unfitted = append(unfitted, resetItem(items[i]))
		}
	}

	return &model.Result{
		Bins:          resultBins,
		UnfittedItems: unfitted,
		Stats:         computeStats(resultBins, items, unfitted),
	}
}
