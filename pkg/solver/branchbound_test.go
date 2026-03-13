package solver

import (
	"context"
	"testing"
	"time"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
)

func pivotFactory() placement.Engine { return placement.NewPivotEngine() }

func TestBBFastOptimal(t *testing.T) {
	// 8 cubes of 10x10x10 fit perfectly in a 20x20x20 bin.
	bb := NewBranchBound(pivotFactory)

	bins := []*model.Bin{model.NewBin("box", 20, 20, 20, 1000)}
	items := make([]*model.Item, 8)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}
	if result.Stats.FittedItems != 8 {
		t.Errorf("FittedItems = %d, want 8", result.Stats.FittedItems)
	}
}

func TestBBFullOptimal(t *testing.T) {
	bb := NewBranchBound(pivotFactory, BBFull())

	bins := []*model.Bin{model.NewBin("box", 20, 20, 20, 1000)}
	items := []*model.Item{
		model.NewItem("a", 20, 10, 10, 1),
		model.NewItem("b", 20, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
		model.NewItem("d", 10, 10, 10, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}
	if result.Stats.FittedItems != 4 {
		t.Errorf("FittedItems = %d, want 4", result.Stats.FittedItems)
	}
}

func TestBBBetterThanGreedy(t *testing.T) {
	bb := NewBranchBound(pivotFactory)

	bins := []*model.Bin{model.NewBin("box", 20, 15, 15, 1000)}
	items := []*model.Item{
		model.NewItem("a", 10, 10, 15, 1),
		model.NewItem("b", 10, 15, 10, 1),
		model.NewItem("c", 15, 5, 10, 1),
		model.NewItem("d", 5, 10, 5, 1),
		model.NewItem("e", 5, 5, 5, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	// Greedy (default order).
	engine := placement.NewPivotEngine()
	greedyBin := model.NewBin("box", 20, 15, 15, 1000)
	greedyPlaced := 0
	for _, item := range items {
		ic := model.NewItem(item.ID, item.Width, item.Height, item.Depth, item.Weight)
		if engine.PlaceItem(greedyBin, ic) {
			greedyPlaced++
		}
	}

	t.Logf("greedy=%d, bb=%d", greedyPlaced, result.Stats.FittedItems)
	if result.Stats.FittedItems < greedyPlaced {
		t.Errorf("B&B (%d) should be >= greedy (%d)", result.Stats.FittedItems, greedyPlaced)
	}
}

func TestBBDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	bb := NewBranchBound(pivotFactory)

	bins := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}
	items := make([]*model.Item, 15)
	for i := range items {
		w := float64(5 + i%10)
		h := float64(3 + (i*3)%8)
		d := float64(4 + (i*7)%9)
		items[i] = model.NewItem("i", w, h, d, 1)
	}

	start := time.Now()
	result, err := bb.Solve(ctx, bins, items)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("took %v, expected ≤500ms", elapsed)
	}
	if result.Stats.FittedItems == 0 {
		t.Error("expected at least some items from greedy seed")
	}
}

func TestBBNoIntersections(t *testing.T) {
	bb := NewBranchBound(pivotFactory)

	bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
	items := make([]*model.Item, 6)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	for _, bin := range result.Bins {
		for i := 0; i < len(bin.Items); i++ {
			for j := i + 1; j < len(bin.Items); j++ {
				if intersection.Intersect(bin.Items[i], bin.Items[j]) {
					t.Errorf("items %d and %d intersect in bin %s", i, j, bin.ID)
				}
			}
		}
	}
}

func TestBBEmptyInputs(t *testing.T) {
	bb := NewBranchBound(pivotFactory)

	t.Run("no bins", func(t *testing.T) {
		result, err := bb.Solve(context.Background(), nil, []*model.Item{model.NewItem("i", 5, 5, 5, 1)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted items")
		}
	})

	t.Run("no items", func(t *testing.T) {
		result, err := bb.Solve(context.Background(), []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted items")
		}
	})
}

func TestBBMultipleBins(t *testing.T) {
	bb := NewBranchBound(pivotFactory)

	bins := []*model.Bin{
		model.NewBin("b1", 10, 10, 10, 100),
		model.NewBin("b2", 10, 10, 10, 100),
	}
	items := []*model.Item{
		model.NewItem("i1", 10, 10, 10, 1),
		model.NewItem("i2", 10, 10, 10, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("FittedItems = %d, want 2", result.Stats.FittedItems)
	}
}

func TestNextPermutation(t *testing.T) {
	a := []int{0, 1, 2}
	count := 1
	for nextPermutation(a) {
		count++
	}
	if count != 6 {
		t.Errorf("3! permutations = %d, want 6", count)
	}
}

func TestNextPermutationSingle(t *testing.T) {
	a := []int{0}
	if nextPermutation(a) {
		t.Error("single element should have no next permutation")
	}
}

func BenchmarkBBFast6Items(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bb := NewBranchBound(pivotFactory)
		bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
		items := make([]*model.Item, 6)
		for j := range items {
			items[j] = model.NewItem("i", 10, 10, 10, 1)
		}
		bb.Solve(context.Background(), bins, items)
	}
}

func BenchmarkBBFast8Items(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bb := NewBranchBound(pivotFactory)
		bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
		items := make([]*model.Item, 8)
		for j := range items {
			w := float64(5 + j%10)
			h := float64(3 + (j*3)%8)
			d := float64(4 + (j*7)%9)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		bb.Solve(context.Background(), bins, items)
	}
}

func BenchmarkBBFull6Items(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bb := NewBranchBound(pivotFactory, BBFull())
		bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
		items := make([]*model.Item, 6)
		for j := range items {
			items[j] = model.NewItem("i", 10, 10, 10, 1)
		}
		bb.Solve(context.Background(), bins, items)
	}
}
