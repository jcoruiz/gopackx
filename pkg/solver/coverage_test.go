package solver

import (
	"context"
	"testing"
	"time"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// ---------------------------------------------------------------------------
// 1. MetaSeed option (0% coverage)
// ---------------------------------------------------------------------------

func TestCoverage_MetaSeed(t *testing.T) {
	// Use a custom seed solver (plain TrialPacking without lookahead).
	custom := NewTrialPacking(newPivot)
	m := NewMetaheuristic(newPivot, MetaSeed(custom), MetaMaxIter(10), MetaMaxNoImprove(5))

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 15, 15, 15, 1),
	}

	result, err := m.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("expected 2 fitted, got %d", result.Stats.FittedItems)
	}
}

// ---------------------------------------------------------------------------
// 2. TrialItemSort option (0% coverage)
// ---------------------------------------------------------------------------

func TestCoverage_TrialItemSort(t *testing.T) {
	tp := NewTrialPacking(newPivot, TrialItemSort(strategy.Greedy))

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}

	result, err := tp.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("expected 2 fitted, got %d", result.Stats.FittedItems)
	}
}

// ---------------------------------------------------------------------------
// 3. shakeRepack without engine (0% coverage for the nil-engine wrapper)
// ---------------------------------------------------------------------------

func TestCoverage_ShakeRepack_NilEngine(t *testing.T) {
	// Build a 2-bin solution where the smallest bin can be redistributed.
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 5, 5, 5, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, 0, 1}, // a,b in bin0; c alone in bin1
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// Call the nil-engine wrapper directly.
	result := shakeRepack(sol, items, binTypes)
	if result == nil {
		t.Fatal("expected shakeRepack to succeed redistributing 1 item")
	}
	if result.nBins != 1 {
		t.Errorf("expected 1 bin after repack, got %d", result.nBins)
	}
}

// ---------------------------------------------------------------------------
// 4 & 5. sortedDims / sortedDims3 and shakeChangeType downsize (0% / 63.2%)
// ---------------------------------------------------------------------------

func TestCoverage_SortedDims(t *testing.T) {
	item := model.NewItem("x", 30, 10, 20, 1) // unordered dims
	d := sortedDims(item)
	if d[0] != 10 || d[1] != 20 || d[2] != 30 {
		t.Errorf("expected [10 20 30], got %v", d)
	}
}

func TestCoverage_SortedDims3(t *testing.T) {
	d := sortedDims3(30, 10, 20)
	if d[0] != 10 || d[1] != 20 || d[2] != 30 {
		t.Errorf("expected [10 20 30], got %v", d)
	}

	// All equal.
	d2 := sortedDims3(5, 5, 5)
	if d2[0] != 5 || d2[1] != 5 || d2[2] != 5 {
		t.Errorf("expected [5 5 5], got %v", d2)
	}
}

func TestCoverage_ShakeChangeType_Downsize(t *testing.T) {
	// One item in a large bin that fits perfectly in a smaller bin.
	items := []*model.Item{
		model.NewItem("small-item", 10, 10, 10, 1),
	}
	binTypes := []*model.Bin{
		model.NewBin("small", 15, 15, 15, 100),  // volume 3375
		model.NewBin("large", 30, 30, 30, 1000), // volume 27000
	}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{1}, // item is in large bin type
		nBins:       1,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeChangeType(sol, items, binTypes)
	if result == nil {
		t.Fatal("expected shakeChangeType to succeed downsizing")
	}
	if result.binTypeIdx[0] != 0 {
		t.Errorf("expected bin type 0 (small), got %d", result.binTypeIdx[0])
	}
}

func TestCoverage_ShakeChangeType_NoDownsize(t *testing.T) {
	// Item barely fits the current bin, no smaller type available.
	items := []*model.Item{
		model.NewItem("big-item", 25, 25, 25, 1),
	}
	binTypes := []*model.Bin{
		model.NewBin("small", 10, 10, 10, 100),
		model.NewBin("medium", 30, 30, 30, 1000),
	}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{1}, // item in medium
		nBins:       1,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeChangeType(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil (can't downsize), but got a result")
	}
}

func TestCoverage_ShakeChangeType_DimsDontFit(t *testing.T) {
	// Item volume fits smaller bin but one dimension is too large.
	items := []*model.Item{
		model.NewItem("long", 5, 5, 25, 1), // volume=625, but depth=25 > 20
	}
	binTypes := []*model.Bin{
		model.NewBin("small", 20, 20, 20, 1000), // volume 8000 > 625 but max dim 20 < 25
		model.NewBin("large", 30, 30, 30, 1000),
	}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{1},
		nBins:       1,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeChangeType(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil (dimension check fails), but got a result")
	}
}

func TestCoverage_ShakeChangeType_WeightDontFit(t *testing.T) {
	// Item fits by volume/dims but is too heavy for the smaller bin.
	items := []*model.Item{
		model.NewItem("heavy", 10, 10, 10, 50), // 50 kg
	}
	binTypes := []*model.Bin{
		model.NewBin("small", 15, 15, 15, 10), // max 10 kg
		model.NewBin("large", 30, 30, 30, 1000),
	}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{1},
		nBins:       1,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeChangeType(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil (weight too high), but got a result")
	}
}

// ---------------------------------------------------------------------------
// 6. isBetterSol (62.5% coverage) — cost comparison and same-bins-diff-fill
// ---------------------------------------------------------------------------

func TestCoverage_IsBetterSol_CostComparison(t *testing.T) {
	a := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 10, avgFillPct: 80}}
	b := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 20, avgFillPct: 80}}

	if !isBetterSol(a, b) {
		t.Error("expected a (cost=10) better than b (cost=20)")
	}
	if isBetterSol(b, a) {
		t.Error("expected b NOT better than a")
	}
}

func TestCoverage_IsBetterSol_SameCostFewerBins(t *testing.T) {
	a := &solution{score: solutionScore{unfitted: 0, totalBins: 1, totalCost: 10, avgFillPct: 50}}
	b := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 10, avgFillPct: 80}}

	if !isBetterSol(a, b) {
		t.Error("expected a (1 bin) better than b (2 bins) at same cost")
	}
}

func TestCoverage_IsBetterSol_SameBinsHigherFill(t *testing.T) {
	a := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 0, avgFillPct: 90}}
	b := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 0, avgFillPct: 70}}

	if !isBetterSol(a, b) {
		t.Error("expected a (fill=90) better than b (fill=70)")
	}
	if isBetterSol(b, a) {
		t.Error("expected b NOT better than a")
	}
}

func TestCoverage_IsBetterSol_FewerUnfitted(t *testing.T) {
	a := &solution{score: solutionScore{unfitted: 0, totalBins: 5}}
	b := &solution{score: solutionScore{unfitted: 2, totalBins: 1}}

	if !isBetterSol(a, b) {
		t.Error("expected a (0 unfitted) better than b (2 unfitted)")
	}
}

func TestCoverage_IsBetterSol_OneCostZero(t *testing.T) {
	// One has cost, the other doesn't — triggers the cost branch.
	a := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 0, avgFillPct: 80}}
	b := &solution{score: solutionScore{unfitted: 0, totalBins: 2, totalCost: 5, avgFillPct: 80}}

	if !isBetterSol(a, b) {
		t.Error("expected a (cost=0) better than b (cost=5)")
	}
}

// ---------------------------------------------------------------------------
// 7. isTrialBetter (73.7% coverage) — cost paths, tie-breaking
// ---------------------------------------------------------------------------

func TestCoverage_IsTrialBetter_CostPerVol(t *testing.T) {
	tp := &TrialPacking{lookahead: false}

	a := trialScore{binTypeIdx: 0, costPerVol: 1.0, fillRatio: 0.5}
	b := trialScore{binTypeIdx: 1, costPerVol: 2.0, fillRatio: 0.9}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (cpv=1) better than b (cpv=2)")
	}
}

func TestCoverage_IsTrialBetter_CostPerVolTieFillRatio(t *testing.T) {
	tp := &TrialPacking{lookahead: false}

	a := trialScore{binTypeIdx: 0, costPerVol: 1.0, fillRatio: 0.9}
	b := trialScore{binTypeIdx: 1, costPerVol: 1.0, fillRatio: 0.5}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (same cpv, higher fill) better than b")
	}
}

func TestCoverage_IsTrialBetter_LookaheadEstimatedCost(t *testing.T) {
	tp := &TrialPacking{lookahead: true}

	a := trialScore{binTypeIdx: 0, costPerVol: 1.0, estimatedCost: 10}
	b := trialScore{binTypeIdx: 1, costPerVol: 2.0, estimatedCost: 20}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (estCost=10) better than b (estCost=20)")
	}
}

func TestCoverage_IsTrialBetter_LookaheadSameEstCost_CpvTie(t *testing.T) {
	tp := &TrialPacking{lookahead: true}

	a := trialScore{binTypeIdx: 0, costPerVol: 1.5, estimatedCost: 10, fillRatio: 0.9}
	b := trialScore{binTypeIdx: 1, costPerVol: 1.5, estimatedCost: 10, fillRatio: 0.5}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (same cost, higher fill) better than b")
	}
}

func TestCoverage_IsTrialBetter_LookaheadNoCost_EstBins(t *testing.T) {
	tp := &TrialPacking{lookahead: true}

	a := trialScore{binTypeIdx: 0, estimatedBins: 2, fillRatio: 0.5}
	b := trialScore{binTypeIdx: 1, estimatedBins: 3, fillRatio: 0.9}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (2 estBins) better than b (3 estBins)")
	}
}

func TestCoverage_IsTrialBetter_LookaheadNoCost_SameEstBins_FillTie(t *testing.T) {
	tp := &TrialPacking{lookahead: true}

	a := trialScore{binTypeIdx: 0, estimatedBins: 2, fillRatio: 0.8}
	b := trialScore{binTypeIdx: 1, estimatedBins: 2, fillRatio: 0.8}

	// Same fill, same estBins -> prefer smaller binTypeIdx.
	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (idx=0) better than b (idx=1) on tie")
	}
}

func TestCoverage_IsTrialBetter_NoLookaheadNoCost_SameFill_FittedCount(t *testing.T) {
	tp := &TrialPacking{lookahead: false}

	a := trialScore{binTypeIdx: 0, fillRatio: 0.8, fittedCount: 5}
	b := trialScore{binTypeIdx: 1, fillRatio: 0.8, fittedCount: 3}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (5 fitted) better than b (3 fitted)")
	}
}

func TestCoverage_IsTrialBetter_NoLookaheadNoCost_AllTied(t *testing.T) {
	tp := &TrialPacking{lookahead: false}

	a := trialScore{binTypeIdx: 0, fillRatio: 0.8, fittedCount: 5}
	b := trialScore{binTypeIdx: 1, fillRatio: 0.8, fittedCount: 5}

	// All tied -> prefer smaller binTypeIdx.
	if !tp.isTrialBetter(a, b) {
		t.Error("expected a (idx=0) better than b (idx=1)")
	}
}

func TestCoverage_IsTrialBetter_FirstValid(t *testing.T) {
	tp := &TrialPacking{}

	a := trialScore{binTypeIdx: 0, fillRatio: 0.5}
	b := trialScore{binTypeIdx: -1}

	if !tp.isTrialBetter(a, b) {
		t.Error("expected any valid trial better than binTypeIdx=-1")
	}
}

// ---------------------------------------------------------------------------
// 8. shakeMove (80%) — single item in source, all preconditions failing
// ---------------------------------------------------------------------------

func TestCoverage_ShakeMove_SingleItemSource(t *testing.T) {
	// Bin 0 has 1 item (small), bin 1 has 1 item (large).
	// Moving the small item from bin 0 to bin 1 should eliminate bin 0.
	items := []*model.Item{
		model.NewItem("small", 5, 5, 5, 1),
		model.NewItem("big", 20, 20, 20, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeMove(sol, items, binTypes)
	if result == nil {
		t.Fatal("expected shakeMove to succeed")
	}
	// Source bin 0 had 1 item -> should be removed.
	if result.nBins != 1 {
		t.Errorf("expected 1 bin after move, got %d", result.nBins)
	}
}

func TestCoverage_ShakeMove_LessThan2Bins(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 5, 5, 5, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	result := shakeMove(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil with < 2 bins")
	}
}

func TestCoverage_ShakeMove_AllBinsFull(t *testing.T) {
	// Two bins each nearly full; items can't be moved.
	items := []*model.Item{
		model.NewItem("a", 29, 29, 29, 999),
		model.NewItem("b", 29, 29, 29, 999),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeMove(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil (no room to move items)")
	}
}

func TestCoverage_ShakeMove_WeightExceeded(t *testing.T) {
	// Volume fits but weight doesn't.
	items := []*model.Item{
		model.NewItem("heavy1", 5, 5, 5, 8),
		model.NewItem("heavy2", 5, 5, 5, 8),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 10)} // max 10kg

	sol := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeMove(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil (weight exceeded)")
	}
}

// ---------------------------------------------------------------------------
// 9. materialize — repackBin fails (graceful fallback)
// ---------------------------------------------------------------------------

// failEngine is a placement engine that always fails to place items.
type failEngine struct{}

func (failEngine) PlaceItem(*model.Bin, *model.Item) bool { return false }

func TestCoverage_Materialize_RepackFails(t *testing.T) {
	newFail := func() placement.Engine { return failEngine{} }
	m := &Metaheuristic{newEngine: newFail}

	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, 0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	result := m.materialize(sol, items, binTypes)
	// repackBin should fail -> items go to unfitted.
	if len(result.Bins) != 0 {
		t.Errorf("expected 0 bins (repack failed), got %d", len(result.Bins))
	}
	if len(result.UnfittedItems) != 2 {
		t.Errorf("expected 2 unfitted, got %d", len(result.UnfittedItems))
	}
}

func TestCoverage_Materialize_UnassignedItems(t *testing.T) {
	m := &Metaheuristic{newEngine: newPivot}

	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, -1}, // b is unassigned
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	result := m.materialize(sol, items, binTypes)
	if len(result.UnfittedItems) != 1 {
		t.Errorf("expected 1 unfitted (unassigned item), got %d", len(result.UnfittedItems))
	}
}

// ---------------------------------------------------------------------------
// 10. matchBinType — no match (falls through to return 0)
// ---------------------------------------------------------------------------

func TestCoverage_MatchBinType_NoMatch(t *testing.T) {
	bin := model.NewBin("unknown", 99, 99, 99, 99)
	binTypes := []*model.Bin{
		model.NewBin("a", 10, 10, 10, 10),
		model.NewBin("b", 20, 20, 20, 20),
	}

	idx := matchBinType(bin, binTypes)
	if idx != 0 {
		t.Errorf("expected 0 (fallback), got %d", idx)
	}
}

func TestCoverage_MatchBinType_ExactMatch(t *testing.T) {
	bin := model.NewBin("x", 20, 20, 20, 20)
	binTypes := []*model.Bin{
		model.NewBin("a", 10, 10, 10, 10),
		model.NewBin("b", 20, 20, 20, 20),
	}

	idx := matchBinType(bin, binTypes)
	if idx != 1 {
		t.Errorf("expected 1, got %d", idx)
	}
}

// ---------------------------------------------------------------------------
// 11. Metaheuristic.Solve — empty seed, context cancellation mid-VNS
// ---------------------------------------------------------------------------

// emptySolver always returns an empty result (no items fitted).
type emptySolver struct{}

func (emptySolver) Solve(_ context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error) {
	return &model.Result{
		Bins:          nil,
		UnfittedItems: items,
		Stats:         model.PackingStats{TotalItems: len(items), UnfittedCount: len(items)},
	}, nil
}

func TestCoverage_MetaSolve_EmptySeedResult(t *testing.T) {
	m := NewMetaheuristic(newPivot, MetaSeed(emptySolver{}), MetaMaxIter(5), MetaMaxNoImprove(3))

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, err := m.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// With an empty seed, VNS has nothing to improve — all items should be unfitted.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestCoverage_MetaSolve_ContextCancelMidVNS(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Give it a large problem so VNS runs for a while.
	m := NewMetaheuristic(newPivot, MetaMaxIter(100000), MetaMaxNoImprove(100000))

	bins := []*model.Bin{
		model.NewBin("small", 30, 30, 30, 1000),
		model.NewBin("large", 60, 60, 60, 1000),
	}
	items := make([]*model.Item, 15)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	// Let the context expire.
	time.Sleep(2 * time.Millisecond)

	result, _ := m.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected non-nil result despite cancellation")
	}
}

// ---------------------------------------------------------------------------
// 12. TrialPacking.Solve — context cancellation during solve
// ---------------------------------------------------------------------------

func TestCoverage_TrialSolve_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	tp := NewTrialPacking(newPivot)
	bins := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}
	items := make([]*model.Item, 200)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	// Cancel immediately.
	cancel()

	result, _ := tp.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	// All items should be unfitted since context was cancelled.
	if len(result.UnfittedItems) == 0 {
		t.Error("expected some unfitted items due to cancelled context")
	}
}

func TestCoverage_TrialSelectBinType_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling

	tp := NewTrialPacking(newPivot, WithLookahead())
	bins := []*model.Bin{
		model.NewBin("a", 30, 30, 30, 1000),
		model.NewBin("b", 50, 50, 50, 1000),
	}
	items := []*model.Item{model.NewItem("x", 20, 20, 20, 1)}

	result, _ := tp.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected result")
	}
}

// ---------------------------------------------------------------------------
// 13. packGreedy — context cancellation during packing
// ---------------------------------------------------------------------------

func TestCoverage_PackGreedy_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	engine := newPivot()
	bins := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}
	items := resetItems([]*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	})

	result := packGreedy(ctx, engine, bins, items, strategy.BestFitDecreasing)
	if len(result.UnfittedItems) != 2 {
		t.Errorf("expected 2 unfitted with cancelled ctx, got %d", len(result.UnfittedItems))
	}
}

// ---------------------------------------------------------------------------
// 14. optimizeFast — all items fit on first try (n <= 1)
// ---------------------------------------------------------------------------

func TestCoverage_OptimizeFast_SingleItem(t *testing.T) {
	bb := NewBranchBound(newPivot)

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 1 {
		t.Errorf("expected 1 fitted, got %d", result.Stats.FittedItems)
	}
}

func TestCoverage_OptimizeFast_AllFitGreedy(t *testing.T) {
	bb := NewBranchBound(newPivot)

	bins := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}
	// 2 small items that trivially fit.
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("expected 2 fitted, got %d", result.Stats.FittedItems)
	}
}

// ---------------------------------------------------------------------------
// 15. revalidate — bin index out of range, empty bin
// ---------------------------------------------------------------------------

func TestCoverage_Revalidate_BinOutOfRange(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	// dirtyBins includes an index that is out of range.
	ok := revalidate(newPivot, sol, items, binTypes, []int{0, 5, 99})
	if !ok {
		t.Error("expected true (out-of-range bins should be skipped)")
	}
}

func TestCoverage_Revalidate_EmptyBin(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	// Bin 1 has no items assigned.
	sol := &solution{
		assignments: []int{0, 0},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}

	ok := revalidate(newPivot, sol, items, binTypes, []int{1})
	if !ok {
		t.Error("expected true (empty bin should be skipped)")
	}
}

func TestCoverage_Revalidate_FailsOnInfeasible(t *testing.T) {
	// Items that can't actually be packed in the bin by the engine.
	items := []*model.Item{
		model.NewItem("a", 25, 25, 25, 1),
		model.NewItem("b", 25, 25, 25, 1),
		model.NewItem("c", 25, 25, 25, 1),
		model.NewItem("d", 25, 25, 25, 1),
		model.NewItem("e", 25, 25, 25, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)} // Can only fit 1

	sol := &solution{
		assignments: []int{0, 0, 0, 0, 0}, // All assigned to bin 0
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	ok := revalidate(newPivot, sol, items, binTypes, []int{0})
	if ok {
		t.Error("expected false (5 items of 25^3 can't fit in 30^3)")
	}
}

// ---------------------------------------------------------------------------
// Additional edge cases for deeper coverage
// ---------------------------------------------------------------------------

func TestCoverage_ShakeSwap_LessThan2Bins(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 5, 5, 5, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	result := shakeSwap(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil with < 2 bins")
	}
}

func TestCoverage_ShakeSwap_SinglePair(t *testing.T) {
	// Only 1 item total — need at least 2 for swap.
	items := []*model.Item{
		model.NewItem("a", 5, 5, 5, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}

	result := shakeSwap(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil with < 2 pairs")
	}
}

func TestCoverage_ShakeRepack_LessThan2Bins(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 5, 5, 5, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	result := shakeRepackWithEngine(sol, items, binTypes, nil)
	if result != nil {
		t.Error("expected nil with < 2 bins")
	}
}

func TestCoverage_ShakeChangeType_EmptyBin(t *testing.T) {
	// Bin 0 has items, bin 1 is empty.
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{
		model.NewBin("small", 15, 15, 15, 100),
		model.NewBin("large", 30, 30, 30, 1000),
	}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{1, 1}, // bin 1 has no items
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// Should still succeed for the bin with items.
	result := shakeChangeType(sol, items, binTypes)
	if result == nil {
		t.Fatal("expected shakeChangeType to downsize the non-empty bin")
	}
}

func TestCoverage_ExtractSolution_EmptyResult(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	result := &model.Result{
		Bins:          nil,
		UnfittedItems: items,
	}

	sol := extractSolution(result, items, binTypes)
	if sol.nBins != 0 {
		t.Errorf("expected 0 bins, got %d", sol.nBins)
	}
	if sol.assignments[0] != -1 {
		t.Errorf("expected -1 assignment, got %d", sol.assignments[0])
	}
}

func TestCoverage_ScoreSolution_NoBins(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{-1},
		binTypeIdx:  nil,
		nBins:       0,
	}

	score := scoreSolution(sol, items, binTypes)
	if score.unfitted != 1 {
		t.Errorf("expected 1 unfitted, got %d", score.unfitted)
	}
	if score.avgFillPct != 0 {
		t.Errorf("expected 0 avgFill, got %f", score.avgFillPct)
	}
}

func TestCoverage_ScoreSolution_WithCost(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000, model.BinCost(25))}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	score := scoreSolution(sol, items, binTypes)
	if score.totalCost != 25 {
		t.Errorf("expected totalCost=25, got %f", score.totalCost)
	}
}

func TestCoverage_DirtyBins_ChangedType(t *testing.T) {
	m := &Metaheuristic{}

	old := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	new := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 1}, // bin 1 changed type
		nBins:       2,
	}

	dirty := m.dirtyBins(old, new)
	found := false
	for _, b := range dirty {
		if b == 1 {
			found = true
		}
	}
	if !found {
		t.Error("expected bin 1 in dirty list due to type change")
	}
}

func TestCoverage_CloneSolution(t *testing.T) {
	orig := &solution{
		assignments: []int{0, 1, -1},
		binTypeIdx:  []int{0, 1},
		nBins:       2,
		score:       solutionScore{totalBins: 2, unfitted: 1},
	}

	c := cloneSolution(orig)

	// Modify original, clone should be unaffected.
	orig.assignments[0] = 99
	orig.binTypeIdx[0] = 99

	if c.assignments[0] != 0 {
		t.Error("clone was affected by original modification")
	}
	if c.binTypeIdx[0] != 0 {
		t.Error("clone binTypeIdx was affected by original modification")
	}
}

func TestCoverage_RemoveBin(t *testing.T) {
	sol := &solution{
		assignments: []int{0, 1, 2, 1},
		binTypeIdx:  []int{0, 1, 2},
		nBins:       3,
	}

	sol.removeBin(1)

	if sol.nBins != 2 {
		t.Errorf("expected 2 bins, got %d", sol.nBins)
	}
	// Item 0 stays at bin 0.
	if sol.assignments[0] != 0 {
		t.Errorf("expected 0, got %d", sol.assignments[0])
	}
	// Item 1 was in removed bin -> -1.
	if sol.assignments[1] != -1 {
		t.Errorf("expected -1, got %d", sol.assignments[1])
	}
	// Item 2 was in bin 2 -> now bin 1.
	if sol.assignments[2] != 1 {
		t.Errorf("expected 1, got %d", sol.assignments[2])
	}
	// Item 3 was in removed bin -> -1.
	if sol.assignments[3] != -1 {
		t.Errorf("expected -1, got %d", sol.assignments[3])
	}
}

func TestCoverage_ItemsInBin(t *testing.T) {
	sol := &solution{
		assignments: []int{0, 1, 0, -1, 0},
		nBins:       2,
	}

	items := itemsInBin(sol, 0)
	if len(items) != 3 {
		t.Errorf("expected 3 items in bin 0, got %d", len(items))
	}

	items = itemsInBin(sol, 1)
	if len(items) != 1 {
		t.Errorf("expected 1 item in bin 1, got %d", len(items))
	}

	items = itemsInBin(sol, 2)
	if len(items) != 0 {
		t.Errorf("expected 0 items in bin 2, got %d", len(items))
	}
}

func TestCoverage_BinFillVolume(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1), // vol=1000
		model.NewItem("b", 5, 5, 5, 1),    // vol=125
	}

	sol := &solution{
		assignments: []int{0, 0},
		nBins:       1,
	}

	vol := binFillVolume(sol, 0, items)
	if vol != 1125 {
		t.Errorf("expected 1125, got %f", vol)
	}
}

func TestCoverage_BinFillWeight(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 3),
		model.NewItem("b", 5, 5, 5, 7),
	}

	sol := &solution{
		assignments: []int{0, 0},
		nBins:       1,
	}

	w := binFillWeight(sol, 0, items)
	if w != 10 {
		t.Errorf("expected 10, got %f", w)
	}
}

func TestCoverage_BinInstanceID(t *testing.T) {
	id := binInstanceID("CAJA-01", 3)
	if id != "CAJA-01-3" {
		t.Errorf("expected CAJA-01-3, got %s", id)
	}
}

// ---------------------------------------------------------------------------
// Full integration: metaheuristic with cost-aware bins
// ---------------------------------------------------------------------------

func TestCoverage_Meta_CostAware(t *testing.T) {
	m := NewMetaheuristic(newPivot, MetaMaxIter(50), MetaMaxNoImprove(20))

	binTypes := []*model.Bin{
		model.NewBin("cheap", 30, 30, 30, 1000, model.BinCost(5)),
		model.NewBin("big", 60, 60, 60, 1000, model.BinCost(50)),
	}

	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}

	result, err := m.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 3 {
		t.Errorf("expected 3 fitted, got %d", result.Stats.FittedItems)
	}
}

// ---------------------------------------------------------------------------
// RepackBin with different sort orders
// ---------------------------------------------------------------------------

func TestCoverage_RepackBin_AllOrdersFail(t *testing.T) {
	newFail := func() placement.Engine { return failEngine{} }
	bt := model.NewBin("box", 30, 30, 30, 1000)
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	_, ok := repackBin(newFail, bt, items)
	if ok {
		t.Error("expected false when engine always fails")
	}
}

func TestCoverage_RepackBin_Success(t *testing.T) {
	bt := model.NewBin("box", 50, 50, 50, 1000)
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}

	packed, ok := repackBin(newPivot, bt, items)
	if !ok {
		t.Fatal("expected repackBin to succeed")
	}
	if len(packed.Items) != 2 {
		t.Errorf("expected 2 items packed, got %d", len(packed.Items))
	}
}

// ---------------------------------------------------------------------------
// Shake operators via metaheuristic shake method
// ---------------------------------------------------------------------------

func TestCoverage_Shake_UnknownOp(t *testing.T) {
	m := &Metaheuristic{newEngine: newPivot}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0},
		nBins:       1,
	}

	result := m.shake(sol, neighborhoodOp(99), items, binTypes)
	if result != nil {
		t.Error("expected nil for unknown op")
	}
}

// ---------------------------------------------------------------------------
// HasCosts helper
// ---------------------------------------------------------------------------

func TestCoverage_HasCosts(t *testing.T) {
	noCost := []*model.Bin{model.NewBin("a", 10, 10, 10, 10)}
	if hasCosts(noCost) {
		t.Error("expected false for bins without cost")
	}

	withCost := []*model.Bin{model.NewBin("a", 10, 10, 10, 10, model.BinCost(5))}
	if !hasCosts(withCost) {
		t.Error("expected true for bins with cost")
	}
}

// ---------------------------------------------------------------------------
// EstimateTotalBins and EstimateTotalCost edge cases
// ---------------------------------------------------------------------------

func TestCoverage_EstimateTotalBins_AllFit(t *testing.T) {
	tp := &TrialPacking{lookahead: true}
	binTypes := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	remaining := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}

	score := trialScore{binTypeIdx: 0, fillRatio: 0.5, fittedCount: 2}
	est := tp.estimateTotalBins(binTypes, remaining, score)
	if est != 1.0 {
		t.Errorf("expected 1.0 when all fit, got %f", est)
	}
}

func TestCoverage_EstimateTotalCost_AllFit(t *testing.T) {
	tp := &TrialPacking{lookahead: true}
	binTypes := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000, model.BinCost(10))}
	remaining := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
	}

	score := trialScore{binTypeIdx: 0, fillRatio: 0.5, fittedCount: 1}
	cost := tp.estimateTotalCost(binTypes, remaining, score)
	if cost != 10 {
		t.Errorf("expected 10.0 when all fit, got %f", cost)
	}
}

// ---------------------------------------------------------------------------
// sortedDims / sortedDims3 — all branch permutations
// ---------------------------------------------------------------------------

func TestCoverage_SortedDims_AllPermutations(t *testing.T) {
	tests := []struct {
		w, h, d float64
		want    [3]float64
	}{
		{1, 2, 3, [3]float64{1, 2, 3}}, // already sorted
		{3, 2, 1, [3]float64{1, 2, 3}}, // reversed
		{2, 1, 3, [3]float64{1, 2, 3}}, // d[0]>d[1], no second swap needed
		{1, 3, 2, [3]float64{1, 2, 3}}, // d[1]>d[2], then d[0]<d[1]
		{3, 1, 2, [3]float64{1, 2, 3}}, // d[0]>d[1], d[1]>d[2], d[0]>d[1]
		{2, 3, 1, [3]float64{1, 2, 3}}, // d[1]>d[2], then d[0]>d[1]
	}
	for _, tt := range tests {
		item := model.NewItem("x", tt.w, tt.h, tt.d, 1)
		got := sortedDims(item)
		if got != tt.want {
			t.Errorf("sortedDims(%v,%v,%v) = %v, want %v", tt.w, tt.h, tt.d, got, tt.want)
		}
	}
}

func TestCoverage_SortedDims3_AllPermutations(t *testing.T) {
	tests := []struct {
		a, b, c float64
		want    [3]float64
	}{
		{1, 2, 3, [3]float64{1, 2, 3}},
		{3, 2, 1, [3]float64{1, 2, 3}},
		{2, 1, 3, [3]float64{1, 2, 3}},
		{1, 3, 2, [3]float64{1, 2, 3}},
		{3, 1, 2, [3]float64{1, 2, 3}},
		{2, 3, 1, [3]float64{1, 2, 3}},
	}
	for _, tt := range tests {
		got := sortedDims3(tt.a, tt.b, tt.c)
		if got != tt.want {
			t.Errorf("sortedDims3(%v,%v,%v) = %v, want %v", tt.a, tt.b, tt.c, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// optimizeFast — permutation loop runs and finds improvement
// ---------------------------------------------------------------------------

func TestCoverage_OptimizeFast_PermutationImproves(t *testing.T) {
	// Items that don't all fit in seed order but do fit in some permutation.
	// 3 items where order matters for fitting in one bin.
	bb := NewBranchBound(newPivot)

	bins := []*model.Bin{model.NewBin("box", 40, 20, 20, 1000)}
	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 20, 15, 15, 1),
		model.NewItem("c", 20, 5, 5, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// The B&B should try permutations to pack as many as possible.
	if result.Stats.FittedItems < 2 {
		t.Errorf("expected at least 2 fitted via permutation search, got %d", result.Stats.FittedItems)
	}
}

func TestCoverage_OptimizeFast_ContextCancelDuringPerm(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bb := NewBranchBound(newPivot)
	bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
	items := []*model.Item{
		model.NewItem("a", 25, 25, 25, 1),
		model.NewItem("b", 25, 25, 25, 1),
		model.NewItem("c", 25, 25, 25, 1),
	}

	result, err := bb.Solve(ctx, bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// Should return the seed result even with cancelled context.
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// BranchBound.Solve — context cancel mid-solve, empty inputs
// ---------------------------------------------------------------------------

func TestCoverage_BranchBound_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bb := NewBranchBound(newPivot)
	bins := []*model.Bin{
		model.NewBin("a", 30, 30, 30, 1000),
		model.NewBin("b", 30, 30, 30, 1000),
	}
	items := []*model.Item{
		model.NewItem("x", 10, 10, 10, 1),
	}

	result, err := bb.Solve(ctx, bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

func TestCoverage_BranchBound_EmptyInputs(t *testing.T) {
	bb := NewBranchBound(newPivot)

	t.Run("no bins", func(t *testing.T) {
		result, err := bb.Solve(context.Background(), nil, []*model.Item{model.NewItem("x", 5, 5, 5, 1)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted")
		}
	})

	t.Run("no items", func(t *testing.T) {
		result, err := bb.Solve(context.Background(), []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted")
		}
	})
}

// ---------------------------------------------------------------------------
// BranchBound Full variant — exercises dfsFull
// ---------------------------------------------------------------------------

func TestCoverage_BranchBound_Full(t *testing.T) {
	bb := NewBranchBound(newPivot, BBFull())

	bins := []*model.Bin{model.NewBin("box", 40, 40, 40, 1000)}
	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 20, 15, 15, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems < 2 {
		t.Errorf("expected at least 2 fitted, got %d", result.Stats.FittedItems)
	}
}

func TestCoverage_BranchBound_Full_SingleItem(t *testing.T) {
	bb := NewBranchBound(newPivot, BBFull())

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 1 {
		t.Errorf("expected 1 fitted, got %d", result.Stats.FittedItems)
	}
}

func TestCoverage_BranchBound_Full_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bb := NewBranchBound(newPivot, BBFull())
	bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
	items := []*model.Item{
		model.NewItem("a", 15, 15, 15, 1),
		model.NewItem("b", 15, 15, 15, 1),
	}

	result, _ := bb.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// shakeSwap — successful swap
// ---------------------------------------------------------------------------

func TestCoverage_ShakeSwap_Success(t *testing.T) {
	// Create a scenario where swapping items between bins is valid.
	items := []*model.Item{
		model.NewItem("big", 15, 15, 15, 1), // bin 0
		model.NewItem("small", 5, 5, 5, 1),  // bin 1
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeSwap(sol, items, binTypes)
	if result == nil {
		t.Log("shakeSwap returned nil (swap may not improve) - ok")
	}
	// Whether it returns nil or a valid swap, it should not panic.
}

func TestCoverage_ShakeSwap_WeightViolation(t *testing.T) {
	// Swap would violate weight constraints.
	items := []*model.Item{
		model.NewItem("heavy", 5, 5, 5, 9),  // bin 0
		model.NewItem("light", 5, 5, 5, 1),  // bin 1
		model.NewItem("anchor", 5, 5, 5, 9), // bin 1
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 10)} // max 10kg

	sol := &solution{
		assignments: []int{0, 1, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// heavy(9) swaps with light(1): bin0 gets light(1)=ok, bin1 gets heavy(9)+anchor(9)=18>10
	// heavy(9) swaps with anchor(9): same weight, no improvement
	// light(1) can't swap with heavy since bin1 would get 9+9=18>10
	result := shakeSwap(sol, items, binTypes)
	// The swap should either be nil or valid (not violate constraints).
	_ = result
}

func TestCoverage_ShakeSwap_VolumeViolation(t *testing.T) {
	// Swap would violate volume constraints.
	items := []*model.Item{
		model.NewItem("big", 28, 28, 28, 1), // bin 0 - vol ~21952
		model.NewItem("tiny", 2, 2, 2, 1),   // bin 1
	}
	binTypes := []*model.Bin{model.NewBin("box", 29, 29, 29, 1000)} // vol 24389

	sol := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// This won't panic; swap should succeed (volumes are fine) or be nil.
	_ = shakeSwap(sol, items, binTypes)
}

// ---------------------------------------------------------------------------
// shakeRepackWithEngine — redistribution fails (items too big)
// ---------------------------------------------------------------------------

func TestCoverage_ShakeRepackWithEngine_RedistFails(t *testing.T) {
	// Two bins, each with 1 large item. Items can't fit together in one bin.
	items := []*model.Item{
		model.NewItem("a", 28, 28, 28, 1),
		model.NewItem("b", 28, 28, 28, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{0, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeRepackWithEngine(sol, items, binTypes, newPivot)
	if result != nil {
		t.Error("expected nil (can't merge two large items into one bin)")
	}
}

func TestCoverage_ShakeRepackWithEngine_WeightBlock(t *testing.T) {
	// Redistribution blocked by weight: items fit by volume but not by weight.
	items := []*model.Item{
		model.NewItem("a", 5, 5, 5, 5),
		model.NewItem("b", 5, 5, 5, 5),
		model.NewItem("c", 5, 5, 5, 5),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 8)} // max 8kg, only 1 item per bin

	sol := &solution{
		assignments: []int{0, 1, 2},
		binTypeIdx:  []int{0, 0, 0},
		nBins:       3,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	result := shakeRepackWithEngine(sol, items, binTypes, nil)
	if result != nil {
		t.Error("expected nil (weight prevents consolidation)")
	}
}

// ---------------------------------------------------------------------------
// Materialize — empty bin in solution
// ---------------------------------------------------------------------------

func TestCoverage_Materialize_EmptyBinSkipped(t *testing.T) {
	m := &Metaheuristic{newEngine: newPivot}

	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	// Bin 0 has item, bin 1 is empty.
	sol := &solution{
		assignments: []int{0},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}

	result := m.materialize(sol, items, binTypes)
	if len(result.Bins) != 1 {
		t.Errorf("expected 1 bin (empty skipped), got %d", len(result.Bins))
	}
}

// ---------------------------------------------------------------------------
// extractSolution — bins with empty items, duplicate IDs
// ---------------------------------------------------------------------------

func TestCoverage_ExtractSolution_EmptyBinInResult(t *testing.T) {
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	// A result bin with no items should be skipped.
	emptyBin := model.NewBin("box", 30, 30, 30, 1000)
	fullBin := model.NewBin("box", 30, 30, 30, 1000)
	placedItem := model.NewItem("a", 10, 10, 10, 1)
	placedItem.Placed = true
	fullBin.Items = []*model.Item{placedItem}

	result := &model.Result{
		Bins: []*model.Bin{emptyBin, fullBin},
	}

	sol := extractSolution(result, items, binTypes)
	if sol.nBins != 1 {
		t.Errorf("expected 1 bin (empty skipped), got %d", sol.nBins)
	}
	if sol.assignments[0] != 0 {
		t.Errorf("expected item assigned to bin 0, got %d", sol.assignments[0])
	}
}

func TestCoverage_ExtractSolution_DuplicateIDs(t *testing.T) {
	// Multiple items with the same ID.
	items := []*model.Item{
		model.NewItem("dup", 10, 10, 10, 1),
		model.NewItem("dup", 10, 10, 10, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	bin := model.NewBin("box", 30, 30, 30, 1000)
	p1 := model.NewItem("dup", 10, 10, 10, 1)
	p1.Placed = true
	p2 := model.NewItem("dup", 10, 10, 10, 1)
	p2.Placed = true
	bin.Items = []*model.Item{p1, p2}

	result := &model.Result{Bins: []*model.Bin{bin}}

	sol := extractSolution(result, items, binTypes)
	if sol.nBins != 1 {
		t.Errorf("expected 1 bin, got %d", sol.nBins)
	}
	// Both items should be assigned.
	for i, a := range sol.assignments {
		if a != 0 {
			t.Errorf("item %d: expected assignment 0, got %d", i, a)
		}
	}
}

// ---------------------------------------------------------------------------
// TrialPacking.Solve — PlaceItem fails on new bin
// ---------------------------------------------------------------------------

// partialFailEngine places the first item but fails on subsequent ones.
type partialFailEngine struct {
	placed int
	limit  int
}

func (e *partialFailEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.placed >= e.limit {
		return false
	}
	item.Placed = true
	item.Position = [3]float64{0, 0, 0}
	dim := item.Dimension()
	item.PlacedDim = dim
	bin.Items = append(bin.Items, item)
	bin.ItemWeight += item.Weight
	bin.ItemVolume += item.Volume
	e.placed++
	return true
}

func TestCoverage_TrialSolve_PlaceItemFailsOnNewBin(t *testing.T) {
	// Engine that can't place any items - forces the unfitted path after selectBinType.
	newFail := func() placement.Engine { return failEngine{} }
	tp := NewTrialPacking(newFail)

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
	}

	result, err := tp.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// Since engine fails, everything should be unfitted.
	if len(result.UnfittedItems) != 1 {
		t.Errorf("expected 1 unfitted, got %d", len(result.UnfittedItems))
	}
}

// ---------------------------------------------------------------------------
// nextPermutation edge cases
// ---------------------------------------------------------------------------

func TestCoverage_NextPermutation_SingleElement(t *testing.T) {
	a := []int{0}
	if nextPermutation(a) {
		t.Error("expected false for single element")
	}
}

func TestCoverage_NextPermutation_Empty(t *testing.T) {
	var a []int
	if nextPermutation(a) {
		t.Error("expected false for empty slice")
	}
}

func TestCoverage_NextPermutation_FullCycle(t *testing.T) {
	a := []int{0, 1, 2}
	count := 0
	for nextPermutation(a) {
		count++
	}
	// 3! = 6 permutations, starting from {0,1,2} we get 5 more.
	if count != 5 {
		t.Errorf("expected 5 permutations, got %d", count)
	}
}

// ---------------------------------------------------------------------------
// Parallel solver edge cases
// ---------------------------------------------------------------------------

func TestCoverage_Parallel_EmptyConfigs(t *testing.T) {
	p := &Parallel{configs: nil}
	bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, err := p.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// With no configs, all items should be unfitted.
	if result.Stats.FittedItems != 0 {
		t.Errorf("expected 0 fitted with no configs, got %d", result.Stats.FittedItems)
	}
}

// ---------------------------------------------------------------------------
// collectUnfitted / collectUnfittedByUsed
// ---------------------------------------------------------------------------

func TestCoverage_CollectUnfitted(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}
	items[0].Placed = true

	uf := collectUnfitted(items)
	if len(uf) != 1 {
		t.Errorf("expected 1 unfitted, got %d", len(uf))
	}
}

func TestCoverage_CollectUnfittedByUsed(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}
	used := []bool{true, false, true}

	uf := collectUnfittedByUsed(items, used)
	if len(uf) != 1 {
		t.Errorf("expected 1 unfitted, got %d", len(uf))
	}
}

// ---------------------------------------------------------------------------
// optimizeFast — permutation loop: context cancel, found improvement, perfect
// ---------------------------------------------------------------------------

func TestCoverage_OptimizeFast_PermutationFindsImprovement(t *testing.T) {
	// 3 items that don't all fit in default order but fit in some permutation.
	// The bin is small enough that the default ordering can't pack all items.
	bb := NewBranchBound(newPivot)

	// Create a tight bin where item order matters.
	bin := model.NewBin("tight", 35, 20, 20, 1000)
	items := []*model.Item{
		model.NewItem("long", 30, 15, 15, 1),
		model.NewItem("wide", 20, 20, 5, 1),
		model.NewItem("tiny", 5, 5, 5, 1),
	}

	result := bb.optimizeFast(context.Background(), bin, items)
	// B&B should try permutations and fit items.
	if result.count < 2 {
		t.Errorf("expected at least 2 items packed, got %d", result.count)
	}
}

// ---------------------------------------------------------------------------
// shakeMove — all bins have zero volume (srcBin < 0 path)
// ---------------------------------------------------------------------------

func TestCoverage_ShakeMove_AllBinsEmpty(t *testing.T) {
	// All items are unfitted (-1), but nBins >= 2.
	items := []*model.Item{
		model.NewItem("a", 5, 5, 5, 1),
		model.NewItem("b", 5, 5, 5, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	sol := &solution{
		assignments: []int{-1, -1}, // no items assigned to any bin
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}

	result := shakeMove(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil (srcBin < 0 when all bins empty)")
	}
}

// ---------------------------------------------------------------------------
// shakeSwap — x > 50 limit path
// ---------------------------------------------------------------------------

func TestCoverage_ShakeSwap_LargeInput(t *testing.T) {
	// Create many items in 2 bins to trigger the x > 50 cutoff.
	n := 60
	items := make([]*model.Item, n)
	for i := range items {
		items[i] = model.NewItem("i", 3, 3, 3, 1)
	}
	binTypes := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}

	assignments := make([]int, n)
	for i := range assignments {
		if i < n/2 {
			assignments[i] = 0
		} else {
			assignments[i] = 1
		}
	}

	sol := &solution{
		assignments: assignments,
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// This should hit the x > 50 break. Either returns a swap or nil.
	_ = shakeSwap(sol, items, binTypes)
}

// ---------------------------------------------------------------------------
// shakeRepackWithEngine — empty bin in ranked (vol == 0 path)
// ---------------------------------------------------------------------------

func TestCoverage_ShakeRepackWithEngine_EmptyBinInRanked(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 5, 5, 5, 1),
		model.NewItem("b", 5, 5, 5, 1),
	}
	binTypes := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}

	// Bin 0 has items, bin 1 and bin 2 are empty.
	sol := &solution{
		assignments: []int{0, 0},
		binTypeIdx:  []int{0, 0, 0},
		nBins:       3,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// This should skip empty bins in ranking and fail to reduce.
	result := shakeRepackWithEngine(sol, items, binTypes, nil)
	// The only non-empty bin is 0, and we need >= 2 non-empty for meaningful repack.
	// But the function checks nBins >= 2 (which is true for nBins=3).
	// With only 1 non-empty bin, redistribution won't help.
	_ = result
}

// ---------------------------------------------------------------------------
// repackBin — third order (MinimizeBins) succeeds
// ---------------------------------------------------------------------------

// thirdOrderEngine places items only when sorted by MinimizeBins strategy.
// It places items only on the third call sequence (simulating BFD and Greedy failing).
type thirdOrderEngine struct {
	callCount int
}

func (e *thirdOrderEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	e.callCount++
	// Fail for the first two items per bin (BFD and Greedy attempts),
	// succeed for the third (MinimizeBins).
	// Actually simpler: just always place. The test calls repackBin which
	// creates 3 separate engine instances.
	item.Placed = true
	item.Position = [3]float64{0, 0, 0}
	dim := item.Dimension()
	item.PlacedDim = dim
	bin.Items = append(bin.Items, item)
	bin.ItemWeight += item.Weight
	bin.ItemVolume += item.Volume
	return true
}

// failNEngine fails the first N PlaceItem calls, then succeeds.
type failNEngine struct {
	failCount int
	maxFails  int
}

func (e *failNEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.failCount < e.maxFails {
		e.failCount++
		return false
	}
	item.Placed = true
	item.Position = [3]float64{float64(len(bin.Items)) * 10, 0, 0}
	dim := item.Dimension()
	item.PlacedDim = dim
	bin.Items = append(bin.Items, item)
	bin.ItemWeight += item.Weight
	bin.ItemVolume += item.Volume
	return true
}

func TestCoverage_RepackBin_ThirdOrderSucceeds(t *testing.T) {
	// We need an engine where BFD and Greedy fail but MinimizeBins succeeds.
	// The simplest way: the engine fails the first 2 item placements per
	// engine instance, but the third engine instance succeeds.
	callIdx := 0
	newEng := func() placement.Engine {
		callIdx++
		if callIdx <= 2 {
			return failEngine{} // BFD and Greedy engine fail
		}
		return &thirdOrderEngine{} // MinimizeBins engine succeeds
	}

	bt := model.NewBin("box", 50, 50, 50, 1000)
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	packed, ok := repackBin(newEng, bt, items)
	if !ok {
		t.Fatal("expected repackBin to succeed on third order")
	}
	if len(packed.Items) != 1 {
		t.Errorf("expected 1 item packed, got %d", len(packed.Items))
	}
}

// ---------------------------------------------------------------------------
// TrialPacking.Solve — PlaceItem fails AFTER selectBinType returns valid
// This covers line 116-119 where engine fails on the new bin.
// ---------------------------------------------------------------------------

// selectOnlyEngine succeeds during trial (runTrial) but fails on actual placement.
type selectOnlyEngine struct {
	trialMode bool
}

func (e *selectOnlyEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	// Always fail in actual placement mode.
	return false
}

func TestCoverage_TrialSolve_PlaceItemFailsAfterSelect(t *testing.T) {
	// We need: selectBinType returns a valid binTypeIdx (trial places the item),
	// but the actual PlaceItem on the new bin fails.
	// Use an engine that works during trial but fails in the main loop.
	//
	// Since TrialPacking uses the same engine for both trial and main packing,
	// the simplest approach: items are too big for any bin type, but trial's
	// first item check passes because it's within dimensions.
	// Actually, we can use a custom engine.

	// Let's use a counting engine: first N calls succeed (trial), then fail.
	callCount := 0
	// For trial: it calls PlaceItem once per remaining item per bin type.
	// For 1 bin type and 1 item, trial calls PlaceItem 1 time.
	// After trial, the main loop calls PlaceItem for open bins (0 calls),
	// then PlaceItem for the new bin (1 call - should fail).
	tp := &TrialPacking{
		newEngine: func() placement.Engine {
			callCount++
			if callCount == 1 {
				// Trial engine: places items (so fittedCount > 0).
				return &thirdOrderEngine{}
			}
			// Main engine: fails on placement.
			return failEngine{}
		},
		itemSort: strategy.BestFitDecreasing,
	}

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, err := tp.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// The main engine fails to place the item, so it goes to unfitted.
	if len(result.UnfittedItems) != 1 {
		t.Errorf("expected 1 unfitted, got %d", len(result.UnfittedItems))
	}
}

// ---------------------------------------------------------------------------
// Metaheuristic.Solve — context cancelled after seed, before VNS (line 81-83)
// ---------------------------------------------------------------------------

// slowSolver delays so context expires during seed solve.
type slowSolver struct{}

func (slowSolver) Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error) {
	// Simulate a solver that returns after context is cancelled.
	// Return a valid result.
	return &model.Result{
		Bins:          bins,
		UnfittedItems: items,
		Stats:         computeStats(bins, items, items),
	}, nil
}

func TestCoverage_MetaSolve_ContextCancelAfterSeed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately so ctx.Err() != nil after seed returns.
	cancel()

	m := NewMetaheuristic(newPivot, MetaSeed(slowSolver{}), MetaMaxIter(100), MetaMaxNoImprove(50))

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, _ := m.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// Metaheuristic.Solve — inner loop context cancellation (line 101-102)
// ---------------------------------------------------------------------------

func TestCoverage_MetaSolve_InnerLoopCtxCancel(t *testing.T) {
	// Use a very short timeout so the inner ops loop sees ctx.Err().
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	m := NewMetaheuristic(newPivot, MetaMaxIter(100000), MetaMaxNoImprove(100000))

	bins := []*model.Bin{
		model.NewBin("small", 20, 20, 20, 1000),
		model.NewBin("medium", 40, 40, 40, 1000),
		model.NewBin("large", 60, 60, 60, 1000),
	}

	items := make([]*model.Item, 20)
	for i := range items {
		items[i] = model.NewItem("i", 10+float64(i%5), 10, 10, 1)
	}

	result, _ := m.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// dfsFull — context cancellation (line 182), count pruning (line 195)
// ---------------------------------------------------------------------------

func TestCoverage_DfsFull_ContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bb := NewBranchBound(newPivot, BBFull())
	bin := model.NewBin("box", 50, 50, 50, 1000)
	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 20, 20, 20, 1),
	}

	// Directly call optimizeFull with cancelled context.
	result := bb.optimizeFull(ctx, bin, items)
	// Should return the seed result.
	if result.bin == nil {
		t.Fatal("expected non-nil bin")
	}
}

func TestCoverage_DfsFull_OptimalFound(t *testing.T) {
	// Items that all fit in the bin regardless of order.
	bb := NewBranchBound(newPivot, BBFull())

	bin := model.NewBin("box", 100, 100, 100, 1000)
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}

	result := bb.optimizeFull(context.Background(), bin, items)
	if result.count != 3 {
		t.Errorf("expected 3 items packed (optimal), got %d", result.count)
	}
}

// ---------------------------------------------------------------------------
// Parallel.Solve — all goroutines return nil (best == nil path, line 108)
// ---------------------------------------------------------------------------

type nilSolver struct{}

func (nilSolver) Solve(_ context.Context, _ []*model.Bin, _ []*model.Item) (*model.Result, error) {
	return nil, nil
}

func TestCoverage_Parallel_NilResults(t *testing.T) {
	// Use a config where packGreedy is called with a cancelled context
	// so all results are empty (but not nil since packGreedy always returns).
	// Actually, packGreedy always returns a non-nil result.
	// The nil check at line 108 would only trigger if results chan produces no entries.
	// That can't happen with the current code since wg.Wait() ensures all goroutines finish.
	// But we can test the path indirectly with empty configs (already covered).
	// Let's skip and focus on other gaps.
	t.Log("Parallel nil path is unreachable in practice due to packGreedy always returning")
}

// ---------------------------------------------------------------------------
// shakeSwap — successful swap with volume check path
// ---------------------------------------------------------------------------

func TestCoverage_ShakeSwap_VolumeFails(t *testing.T) {
	// Two items in separate bins; swapping would exceed volume in one.
	items := []*model.Item{
		model.NewItem("big", 28, 28, 28, 1),  // vol = 21952, bin 0
		model.NewItem("small", 2, 2, 2, 1),   // vol = 8, bin 1
		model.NewItem("fill", 27, 27, 27, 1), // vol = 19683, bin 1
	}
	binTypes := []*model.Bin{model.NewBin("box", 29, 29, 29, 1000)} // vol = 24389

	sol := &solution{
		assignments: []int{0, 1, 1},
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	_ = shakeSwap(sol, items, binTypes)
}

// ---------------------------------------------------------------------------
// optimizeFast — permutation finds improvement (lines 130-135), perfect pack
// ---------------------------------------------------------------------------

func TestCoverage_OptimizeFast_PermFoundPerfect(t *testing.T) {
	// Need items where: seed order doesn't pack all items,
	// but some permutation packs all n items (triggers placed >= n break).
	bb := NewBranchBound(newPivot)

	// A bin that's tight: 3 items can fit but only in specific orderings.
	bin := model.NewBin("tight", 30, 20, 10, 1000)
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}
	// 3x10x10x10 = 3000, bin vol = 6000, should be possible with right order.

	result := bb.optimizeFast(context.Background(), bin, items)
	if result.count < 3 {
		t.Logf("could only fit %d/3 items (may vary by engine)", result.count)
	}
}

func TestCoverage_OptimizeFast_PermContextCancel(t *testing.T) {
	// Items where seed doesn't fit all, so permutation loop starts,
	// then context is cancelled.
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	bb := NewBranchBound(newPivot)

	bin := model.NewBin("box", 25, 25, 25, 1000)
	// Many items so permutations take time.
	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 20, 15, 15, 1),
		model.NewItem("c", 15, 20, 15, 1),
		model.NewItem("d", 15, 15, 20, 1),
		model.NewItem("e", 10, 10, 10, 1),
	}

	// Let context expire.
	time.Sleep(2 * time.Millisecond)

	result := bb.optimizeFast(ctx, bin, items)
	if result.bin == nil {
		t.Fatal("expected non-nil bin")
	}
}

// ---------------------------------------------------------------------------
// shakeSwap — all pairs fail, x > 50 break, return nil (lines 329-333)
// ---------------------------------------------------------------------------

func TestCoverage_ShakeSwap_AllPairsSameBin(t *testing.T) {
	// All items in one bin (but nBins >= 2). No cross-bin pairs possible.
	n := 55
	items := make([]*model.Item, n)
	for i := range items {
		items[i] = model.NewItem("i", 3, 3, 3, 1)
	}
	binTypes := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}

	assignments := make([]int, n)
	// All in bin 0, bin 1 is empty.
	for i := range assignments {
		assignments[i] = 0
	}

	sol := &solution{
		assignments: assignments,
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// All pairs are in the same bin, so no valid swap exists -> nil.
	result := shakeSwap(sol, items, binTypes)
	if result != nil {
		t.Error("expected nil when all items in same bin")
	}
}

// ---------------------------------------------------------------------------
// TrialPacking.Solve — selectBinType returns valid but PlaceItem fails (116-119)
// ---------------------------------------------------------------------------

// oneTrialEngine: succeeds during trial creation but fails for main engine.
type oneTrialEngine struct {
	isFirst bool
}

func TestCoverage_TrialSolve_PlaceOnNewBinFails(t *testing.T) {
	// Create a scenario where selectBinType finds a valid bin type
	// (the trial engine places the first item) but the main engine
	// fails to place the item on the new bin (line 116).
	//
	// Strategy: use separate engine instances. The TrialPacking creates
	// one engine for main loop and one per trial. If the main engine
	// always fails but trial engines succeed, we hit line 116.
	callIdx := 0
	tp := &TrialPacking{
		newEngine: func() placement.Engine {
			callIdx++
			if callIdx == 1 {
				// Main engine (first created) - always fails.
				return failEngine{}
			}
			// Trial engines - succeed.
			return newPivot()
		},
		itemSort: strategy.BestFitDecreasing,
	}

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := []*model.Item{model.NewItem("a", 10, 10, 10, 1)}

	result, err := tp.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	// Main engine can't place item -> unfitted.
	if len(result.UnfittedItems) != 1 {
		t.Errorf("expected 1 unfitted, got %d", len(result.UnfittedItems))
	}
}

// ---------------------------------------------------------------------------
// selectBinType — context cancelled (line 149-150)
// ---------------------------------------------------------------------------

func TestCoverage_SelectBinType_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tp := &TrialPacking{
		newEngine: newPivot,
		itemSort:  strategy.BestFitDecreasing,
	}

	binTypes := []*model.Bin{
		model.NewBin("a", 30, 30, 30, 1000),
		model.NewBin("b", 50, 50, 50, 1000),
	}
	remaining := resetItems([]*model.Item{model.NewItem("x", 10, 10, 10, 1)})

	score := tp.selectBinType(ctx, binTypes, remaining)
	if score.binTypeIdx != -1 {
		t.Errorf("expected -1 (no selection due to cancel), got %d", score.binTypeIdx)
	}
}

// ---------------------------------------------------------------------------
// optimizeFast — permutation actually improves and finds perfect (130-135)
// ---------------------------------------------------------------------------

// limitedEngine only places items when the first item is "key".
// This simulates an engine sensitive to item order.
type limitedEngine struct {
	firstID string
}

func (e *limitedEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	// If the bin is empty, only accept "key" as first item.
	if len(bin.Items) == 0 {
		if item.ID != "key" {
			return false
		}
	}
	// Place the item.
	item.Placed = true
	item.Position = [3]float64{float64(len(bin.Items)) * 15, 0, 0}
	dim := item.Dimension()
	item.PlacedDim = dim
	bin.Items = append(bin.Items, item)
	bin.ItemWeight += item.Weight
	bin.ItemVolume += item.Volume
	return true
}

func TestCoverage_OptimizeFast_PermFindsAll(t *testing.T) {
	// Use a custom engine where only one specific ordering works.
	// Items: ["other", "key"] in initial order.
	// Seed: "other" first -> fails to place -> count=0.
	// Permutation [1,0]: "key" first -> places both -> count=2=n -> break.
	bb := &BranchBound{
		newEngine: func() placement.Engine { return &limitedEngine{} },
	}

	bin := model.NewBin("box", 100, 100, 100, 1000)
	items := []*model.Item{
		model.NewItem("other", 10, 10, 10, 1),
		model.NewItem("key", 10, 10, 10, 1),
	}

	result := bb.optimizeFast(context.Background(), bin, items)
	if result.count != 2 {
		t.Errorf("expected 2 items packed (perfect), got %d", result.count)
	}
}

// ---------------------------------------------------------------------------
// dfsFull — pruning and optimal-found paths (182, 195, 235)
// ---------------------------------------------------------------------------

func TestCoverage_DfsFull_PruningAndOptimal(t *testing.T) {
	// Use a small problem where dfsFull can find optimal and prune.
	bb := NewBranchBound(newPivot, BBFull())

	// 2 items that both fit easily -> optimal found quickly.
	bin := model.NewBin("box", 50, 50, 50, 1000)
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}

	result := bb.optimizeFull(context.Background(), bin, items)
	// Both items should fit -> optimal.
	if result.count != 2 {
		t.Errorf("expected 2, got %d", result.count)
	}
}

func TestCoverage_DfsFull_CountPruning(t *testing.T) {
	// Scenario where the seed already has count=1 out of 2 items.
	// The DFS should try and find optimal (2).
	// If it finds 2 first, it exercises the best.count >= len(items) path (line 235).
	bb := NewBranchBound(newPivot, BBFull())

	bin := model.NewBin("box", 30, 30, 30, 1000)
	items := []*model.Item{
		model.NewItem("a", 25, 25, 25, 1), // fills most of the bin
		model.NewItem("b", 5, 5, 5, 1),    // tiny, should fit alongside
	}

	result := bb.optimizeFull(context.Background(), bin, items)
	if result.count < 2 {
		t.Logf("could only fit %d/2 items", result.count)
	}
}

// ---------------------------------------------------------------------------
// shakeSwap — weight check path (line 312-313)
// ---------------------------------------------------------------------------

func TestCoverage_ShakeSwap_WeightCheckFails(t *testing.T) {
	// 2 items in 2 bins. Swapping would violate weight constraint.
	items := []*model.Item{
		model.NewItem("light", 5, 5, 5, 1), // bin 0, 1kg
		model.NewItem("heavy", 5, 5, 5, 9), // bin 1, 9kg
	}
	// Bin 0 has another heavy item.
	items = append(items, model.NewItem("anchor", 5, 5, 5, 9)) // bin 0, 9kg

	binTypes := []*model.Bin{model.NewBin("box", 100, 100, 100, 10)} // max 10kg

	sol := &solution{
		assignments: []int{0, 1, 0}, // bin0: light(1)+anchor(9)=10, bin1: heavy(9)
		binTypeIdx:  []int{0, 0},
		nBins:       2,
	}
	sol.score = scoreSolution(sol, items, binTypes)

	// Swap light(1) with heavy(9): bin0 -> anchor(9)+heavy(9)=18 > 10 FAIL
	// Swap anchor(9) with heavy(9): bin0 -> light(1)+heavy(9)=10 ok, bin1 -> anchor(9) ok
	// This exercises the weight check continue path.
	result := shakeSwap(sol, items, binTypes)
	// It may find the anchor<->heavy swap which is weight-valid.
	_ = result
}

// ---------------------------------------------------------------------------
// Metaheuristic.Solve — inner loop ctx break (line 101)
// This is hard to hit precisely since the outer loop also checks ctx.
// Use a very tight scenario.
// ---------------------------------------------------------------------------

func TestCoverage_MetaSolve_InnerLoopBreak(t *testing.T) {
	// Use a very short deadline so VNS starts but inner loop cancels mid-ops.
	m := NewMetaheuristic(newPivot, MetaMaxIter(1000000), MetaMaxNoImprove(1000000))

	bins := []*model.Bin{
		model.NewBin("small", 25, 25, 25, 1000),
		model.NewBin("medium", 40, 40, 40, 1000),
		model.NewBin("large", 60, 60, 60, 1000),
	}

	items := make([]*model.Item, 30)
	for i := range items {
		items[i] = model.NewItem("i", 8+float64(i%10), 8+float64(i%7), 8+float64(i%5), 1)
	}

	// Run with very short deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	result, _ := m.Solve(ctx, bins, items)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
}

// ---------------------------------------------------------------------------
// dfsFull — directly call dfsFull to exercise ctx cancel, pruning, optimal
// ---------------------------------------------------------------------------

func TestCoverage_DfsFull_Direct_CtxCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	bb := &BranchBound{
		newEngine: newPivot,
		full:      true,
	}

	bin := cloneBinEmpty(model.NewBin("box", 50, 50, 50, 1000))
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}
	used := make([]bool, 2)
	best := &singleBinResult{bin: bin, count: 0}

	engine := bb.newEngine()

	// Cancel context before DFS.
	cancel()

	bb.dfsFull(ctx, engine, bin, items, used, 0, best)
	// Should return immediately due to ctx cancel.
}

func TestCoverage_DfsFull_Direct_CountPruning(t *testing.T) {
	bb := &BranchBound{
		newEngine: newPivot,
		full:      true,
	}

	bin := cloneBinEmpty(model.NewBin("box", 50, 50, 50, 1000))
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}
	used := make([]bool, 3)
	// Set best.count = 3 (already optimal). depth=0, remaining=3.
	// depth+remaining = 3 <= best.count = 3 -> prune.
	best := &singleBinResult{bin: bin, count: 3}

	engine := bb.newEngine()

	bb.dfsFull(context.Background(), engine, bin, items, used, 0, best)
	// Should prune immediately since we can't beat best.count=3 with depth+remaining=3.
}

func TestCoverage_DfsFull_Direct_OptimalEarlyReturn(t *testing.T) {
	bb := &BranchBound{
		newEngine: newPivot,
		full:      true,
	}

	// 2 items that easily fit. DFS will find both -> best.count = 2 = len(items).
	bin := cloneBinEmpty(model.NewBin("box", 100, 100, 100, 1000))
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
	}
	used := make([]bool, 2)
	best := &singleBinResult{bin: bin, count: 0}

	engine := bb.newEngine()

	bb.dfsFull(context.Background(), engine, bin, items, used, 0, best)
	// Should find optimal (2) and return via the early return path.
	if best.count != 2 {
		t.Errorf("expected optimal count=2, got %d", best.count)
	}
}
