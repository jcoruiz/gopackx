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

func TestBBFullDFS(t *testing.T) {
	// The greedy seed places items in given order. With the large item first,
	// greedy only fits 1. DFS explores other orderings and finds a better packing.
	bb := NewBranchBound(pivotFactory, BBFull())

	bins := []*model.Bin{model.NewBin("box", 10, 10, 10, 1000)}
	items := []*model.Item{
		model.NewItem("big", 8, 8, 8, 1),     // greedy places this first, blocks others
		model.NewItem("s1", 5, 5, 5, 1),
		model.NewItem("s2", 5, 5, 5, 1),
		model.NewItem("s3", 5, 5, 5, 1),
	}

	result, err := bb.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatalf("Solve() error: %v", err)
	}

	// Greedy with big first = 1 item. DFS should find that placing the 3 small
	// items first fits more total items (3 or more vs 1).
	if result.Stats.FittedItems < 2 {
		t.Errorf("FittedItems = %d, want >= 2 (DFS should beat greedy seed of 1)", result.Stats.FittedItems)
	}
}

func TestBBFullBetterThanFast(t *testing.T) {
	// Scenario where rotation selection matters — full should be >= fast.
	binSpec := model.NewBin("box", 30, 20, 15, 1000)

	items := []*model.Item{
		model.NewItem("a", 15, 20, 10, 1),
		model.NewItem("b", 10, 15, 10, 1),
		model.NewItem("c", 15, 5, 15, 1),
		model.NewItem("d", 5, 10, 5, 1),
	}

	bbFast := NewBranchBound(pivotFactory)
	fastResult, _ := bbFast.Solve(context.Background(),
		[]*model.Bin{model.NewBin(binSpec.ID, binSpec.Width, binSpec.Height, binSpec.Depth, binSpec.MaxWeight)},
		resetItems(items))

	bbFull := NewBranchBound(pivotFactory, BBFull())
	fullResult, _ := bbFull.Solve(context.Background(),
		[]*model.Bin{model.NewBin(binSpec.ID, binSpec.Width, binSpec.Height, binSpec.Depth, binSpec.MaxWeight)},
		resetItems(items))

	if fullResult.Stats.FittedItems < fastResult.Stats.FittedItems {
		t.Errorf("Full (%d) should be >= Fast (%d)",
			fullResult.Stats.FittedItems, fastResult.Stats.FittedItems)
	}
}

func TestBBFullDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	bb := NewBranchBound(pivotFactory, BBFull())

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 10000)}
	items := make([]*model.Item, 10)
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

func TestCloneSnapshotBin(t *testing.T) {
	bin := model.NewBin("box", 20, 20, 20, 100)
	a := model.NewItem("a", 10, 10, 10, 5, model.ItemFragile())
	b := model.NewItem("b", 10, 10, 10, 3)
	bin.PlaceItem(a)
	bin.PlaceItem(b)

	snap := snapshotBin(bin)

	// Verify deep copy.
	if snap == bin {
		t.Error("snapshot should be a different pointer")
	}
	if len(snap.Items) != 2 {
		t.Fatalf("snapshot Items count = %d, want 2", len(snap.Items))
	}
	if snap.Items[0] == bin.Items[0] {
		t.Error("snapshot items should be different pointers")
	}
	if snap.Items[0].ID != "a" || snap.Items[1].ID != "b" {
		t.Error("snapshot items should preserve IDs")
	}
	if snap.ItemWeight != bin.ItemWeight {
		t.Errorf("snapshot ItemWeight = %f, want %f", snap.ItemWeight, bin.ItemWeight)
	}
	if len(snap.AABBData) != len(bin.AABBData) {
		t.Errorf("snapshot AABBData len = %d, want %d", len(snap.AABBData), len(bin.AABBData))
	}
	if len(snap.FragileIdxs) != len(bin.FragileIdxs) {
		t.Errorf("snapshot FragileIdxs = %v, want %v", snap.FragileIdxs, bin.FragileIdxs)
	}

	// Mutating snapshot should not affect original.
	snap.Items[0].ID = "modified"
	if bin.Items[0].ID == "modified" {
		t.Error("modifying snapshot should not affect original bin")
	}
}

func TestCloneResetItem(t *testing.T) {
	item := model.NewItem("orig", 10, 20, 30, 5, model.ItemFragile(), model.ItemPriority(2))
	item.Placed = true
	item.Position = [3]float64{1, 2, 3}
	item.RotationType = model.RotationHWD

	reset := resetItem(item)

	if reset == item {
		t.Error("resetItem should return a new pointer")
	}
	if reset.Placed {
		t.Error("reset Placed should be false")
	}
	if reset.Position != ([3]float64{}) {
		t.Errorf("reset Position = %v, want [0 0 0]", reset.Position)
	}
	if reset.RotationType != 0 {
		t.Error("reset RotationType should be 0")
	}
	// Properties should be preserved.
	if reset.ID != "orig" || !reset.Fragile || reset.Priority != 2 {
		t.Error("resetItem should preserve item properties")
	}
	if len(reset.AllowedRotations) != len(item.AllowedRotations) {
		t.Error("resetItem should copy AllowedRotations")
	}
	// Must be a copy, not shared.
	reset.AllowedRotations[0] = model.RotationWDH
	if item.AllowedRotations[0] == model.RotationWDH {
		t.Error("resetItem AllowedRotations should be an independent copy")
	}
}

func TestCollectUnfittedByUsed(t *testing.T) {
	items := []*model.Item{
		model.NewItem("a", 10, 10, 10, 1),
		model.NewItem("b", 10, 10, 10, 1),
		model.NewItem("c", 10, 10, 10, 1),
	}
	used := []bool{true, false, true}

	unfitted := collectUnfittedByUsed(items, used)

	if len(unfitted) != 1 {
		t.Fatalf("unfitted count = %d, want 1", len(unfitted))
	}
	if unfitted[0].ID != "b" {
		t.Errorf("unfitted[0].ID = %q, want %q", unfitted[0].ID, "b")
	}
	if unfitted[0].Placed {
		t.Error("unfitted items should have Placed = false")
	}
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
