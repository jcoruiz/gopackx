package solver

import (
	"context"
	"errors"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// cancelAfter cancels a context once its engines have been asked to place
// n items in total, so tests can stop a solver at an exact point.
type cancelAfter struct {
	n      int
	calls  *int
	cancel context.CancelFunc
}

func (c cancelAfter) factory() func() placement.Engine {
	return func() placement.Engine { return &cancelAfterEngine{c, placement.NewPivotEngine()} }
}

type cancelAfterEngine struct {
	c     cancelAfter
	inner placement.Engine
}

func (e *cancelAfterEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	*e.c.calls++
	if *e.c.calls >= e.c.n {
		e.c.cancel()
	}
	return e.inner.PlaceItem(bin, item)
}

func newCancelAfter(n int) (context.Context, cancelAfter) {
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	return ctx, cancelAfter{n, &calls, cancel}
}

func TestSolversRejectInvalidInput(t *testing.T) {
	bins := []*model.Bin{model.NewBin("b", 10, 10, 10, 0)}
	items := []*model.Item{model.NewItem("bad", 1, 1, 0, 1)}
	for name, s := range map[string]Solver{
		"TrialPacking":  NewTrialPacking(newPivot),
		"Metaheuristic": NewMetaheuristic(newPivot),
		"BranchBound":   NewBranchBound(newPivot),
		"Parallel":      NewParallel(),
	} {
		res, err := s.Solve(context.Background(), bins, items)
		if !errors.Is(err, model.ErrInvalidInput) || res != nil {
			t.Errorf("%s: Solve = %v, %v; want nil and ErrInvalidInput", name, res, err)
		}
	}
}

func cubes(n int, size float64) []*model.Item {
	items := make([]*model.Item, n)
	for i := range items {
		items[i] = model.NewItem("c"+string(rune('a'+i)), size, size, size, 1)
	}
	return items
}

// A permutation interrupted by the deadline is discarded and the error
// reported; later bins are not searched.
func TestBranchBoundStopsInsideAPermutation(t *testing.T) {
	// Two of the three cubes fit, so the seed leaves one out and the search
	// goes on to the permutations; the 4th placement is in the first one.
	ctx, c := newCancelAfter(4)
	bins := []*model.Bin{model.NewBin("a", 10, 5, 5, 0), model.NewBin("b", 10, 5, 5, 0)}
	res, err := NewBranchBound(c.factory()).Solve(ctx, bins, cubes(3, 5))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if len(res.Bins) != 1 || res.Stats.FittedItems != 2 {
		t.Errorf("got %d bins with %d items, want the seed's 2 items in the first bin only", len(res.Bins), res.Stats.FittedItems)
	}
}

func TestBranchBoundFullStopsDeepInTheSearch(t *testing.T) {
	ctx, c := newCancelAfter(5)
	bins := []*model.Bin{model.NewBin("a", 10, 5, 5, 0)}
	res, err := NewBranchBound(c.factory(), BBFull()).Solve(ctx, bins, cubes(3, 5))
	if !errors.Is(err, context.Canceled) || res == nil {
		t.Errorf("Solve = %v, %v; want a result and context.Canceled", res, err)
	}
}

// Items that all fit in the first bin leave the other bins untouched.
func TestBranchBoundStopsWhenEverythingFits(t *testing.T) {
	bins := []*model.Bin{model.NewBin("a", 10, 10, 10, 0), model.NewBin("b", 10, 10, 10, 0)}
	res, err := NewBranchBound(newPivot).Solve(context.Background(), bins, cubes(2, 5))
	if err != nil || len(res.Bins) != 1 || res.Stats.FittedItems != 2 {
		t.Errorf("Solve = %+v, %v; want both items in one bin", res, err)
	}
}

// A trial cut short by the deadline still ends the search with the error.
func TestTrialPackingStopsInsideATrial(t *testing.T) {
	ctx, c := newCancelAfter(2)
	types := []*model.Bin{model.NewBin("box", 10, 10, 10, 0)}
	res, err := NewTrialPacking(c.factory()).Solve(ctx, types, cubes(4, 5))
	if !errors.Is(err, context.Canceled) || res == nil {
		t.Errorf("Solve = %v, %v; want a result and context.Canceled", res, err)
	}
}

// The lookahead estimate ignores weight when a type has no weight limit.
func TestLookaheadWithATypeWithoutWeightLimit(t *testing.T) {
	types := []*model.Bin{model.NewBin("limited", 10, 10, 10, 5), model.NewBin("free", 10, 10, 10, 0)}
	res, err := NewTrialPacking(newPivot, WithLookahead()).Solve(context.Background(), types, cubes(6, 5))
	if err != nil || res.Stats.FittedItems != 6 {
		t.Errorf("Solve = %+v, %v; want every item packed", res, err)
	}
}

func TestParallelReportsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	bins := []*model.Bin{model.NewBin("b", 10, 10, 10, 0)}
	res, err := NewParallel(WithConfig(newPivot, strategy.BestFitDecreasing)).Solve(ctx, bins, cubes(2, 5))
	if !errors.Is(err, context.Canceled) || res == nil || res.Stats.UnfittedCount != 2 {
		t.Errorf("Solve = %+v, %v; want both items unfitted and context.Canceled", res, err)
	}
}

func TestMetaRandomSeedIsUsedAndRepeatable(t *testing.T) {
	m := NewMetaheuristic(newPivot, MetaRandomSeed(42))
	if m.randomSeed != 42 {
		t.Fatalf("randomSeed = %d, want 42", m.randomSeed)
	}
	types := []*model.Bin{model.NewBin("S", 12, 12, 12, 0), model.NewBin("L", 20, 20, 20, 0)}
	items := func() []*model.Item {
		out := make([]*model.Item, 12)
		for i := range out {
			out[i] = model.NewItem("i"+string(rune('a'+i)), float64(3+i%5), float64(4+i%3), float64(5+i%4), 1)
		}
		return out
	}
	r1, _ := m.Solve(context.Background(), types, items())
	r2, _ := m.Solve(context.Background(), types, items())
	if r1.Stats != r2.Stats {
		t.Errorf("same seed, different results: %+v vs %+v", r1.Stats, r2.Stats)
	}
}
