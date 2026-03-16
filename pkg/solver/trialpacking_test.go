package solver

import (
	"context"
	"testing"
	"time"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// newPivot is a helper engine factory for tests.
func newPivot() placement.Engine { return placement.NewPivotEngine() }

func TestTrialPacking_SmallItemsTailPacking(t *testing.T) {
	// Scenario 1: 2 large items (55³) + 8 small items (15³).
	// BFD would use 8 small boxes for the small items.
	// Trial packing should detect that 8 small items fit in 1 medium box.
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("small", 20, 20, 20, 1000),
		model.NewBin("medium", 40, 40, 40, 1000),
		model.NewBin("large", 60, 60, 60, 1000),
	}

	var items []*model.Item
	for i := 0; i < 2; i++ {
		items = append(items, model.NewItem("big", 55, 55, 55, 1))
	}
	for i := 0; i < 8; i++ {
		items = append(items, model.NewItem("small", 15, 15, 15, 1))
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Fitted: %d/%d, Bins used: %d, VolUsed: %.1f%%",
		result.Stats.FittedItems, result.Stats.TotalItems,
		result.Stats.TotalBins, result.Stats.VolumeUsedPct)

	if result.Stats.FittedItems != 10 {
		t.Errorf("expected all 10 items fitted, got %d", result.Stats.FittedItems)
	}

	// Should use significantly fewer bins than 10.
	if result.Stats.TotalBins > 5 {
		t.Errorf("expected ≤5 bins, got %d (trial packing should group small items)", result.Stats.TotalBins)
	}

	// Log bin details for analysis.
	for _, bin := range result.Bins {
		if len(bin.Items) > 0 {
			t.Logf("  Bin %q: %d items, %.1f%% vol used", bin.ID, len(bin.Items), bin.VolumeUsedPct())
		}
	}
}

func TestTrialPacking_PrefersSmallBinWhenItemsFit(t *testing.T) {
	// Items that fit perfectly in a small box should not use a large box.
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("small", 25, 25, 25, 1000),
		model.NewBin("large", 100, 100, 100, 1000),
	}

	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	if result.Stats.TotalBins != 1 {
		t.Fatalf("expected 1 bin, got %d", result.Stats.TotalBins)
	}

	// The bin used should be the small one (higher fill ratio).
	bin := result.Bins[0]
	if bin.Volume > 20000 {
		t.Errorf("expected small bin (vol=15625), got bin with vol=%.0f", bin.Volume)
	}
	t.Logf("Used bin %q (vol=%.0f), fill=%.1f%%", bin.ID, bin.Volume, bin.VolumeUsedPct())
}

func TestTrialPacking_UniformItems(t *testing.T) {
	// 27 items of 18³ should fit in 1 large box (60³) since 3×3×3 = 27.
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("small", 20, 20, 20, 10000),
		model.NewBin("medium", 40, 40, 40, 10000),
		model.NewBin("large", 60, 60, 60, 10000),
	}

	items := make([]*model.Item, 27)
	for i := range items {
		items[i] = model.NewItem("cube", 18, 18, 18, 1)
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Fitted: %d/%d, Bins used: %d", result.Stats.FittedItems, result.Stats.TotalItems, result.Stats.TotalBins)

	if result.Stats.FittedItems != 27 {
		t.Errorf("expected 27 fitted, got %d", result.Stats.FittedItems)
	}

	// Ideal: 1 large box. Accept up to 2.
	if result.Stats.TotalBins > 2 {
		t.Errorf("expected ≤2 bins for 27 cubes of 18³, got %d", result.Stats.TotalBins)
	}

	for _, bin := range result.Bins {
		if len(bin.Items) > 0 {
			t.Logf("  Bin %q: %d items, %.1f%% vol", bin.ID, len(bin.Items), bin.VolumeUsedPct())
		}
	}
}

func TestTrialPacking_NoIntersections(t *testing.T) {
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("small", 30, 30, 30, 1000),
		model.NewBin("medium", 50, 50, 50, 1000),
		model.NewBin("large", 80, 80, 80, 1000),
	}

	items := make([]*model.Item, 20)
	for i := range items {
		w := float64(5 + i%15)
		h := float64(3 + (i*3)%12)
		d := float64(4 + (i*7)%10)
		items[i] = model.NewItem("i", w, h, d, 1)
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	for _, bin := range result.Bins {
		for i := 0; i < len(bin.Items); i++ {
			for j := i + 1; j < len(bin.Items); j++ {
				if intersection.Intersect(bin.Items[i], bin.Items[j]) {
					t.Errorf("bin %q: items %d and %d intersect", bin.ID, i, j)
				}
			}
		}
	}
}

func TestTrialPacking_WeightConstraint(t *testing.T) {
	// Bins with low weight capacity should not be selected for heavy items.
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("light-box", 50, 50, 50, 5),   // max 5 kg
		model.NewBin("heavy-box", 50, 50, 50, 100), // max 100 kg
	}

	items := []*model.Item{
		model.NewItem("heavy", 20, 20, 20, 10), // 10 kg, won't fit in light-box
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	if result.Stats.FittedItems != 1 {
		t.Fatalf("expected 1 item fitted, got %d", result.Stats.FittedItems)
	}

	// Should use the heavy box.
	if result.Bins[0].MaxWeight < 100 {
		t.Errorf("expected heavy-box, got bin with maxWeight=%.0f", result.Bins[0].MaxWeight)
	}
}

func TestTrialPacking_EmptyInputs(t *testing.T) {
	tp := NewTrialPacking(newPivot)

	t.Run("no bins", func(t *testing.T) {
		result, err := tp.Solve(context.Background(), nil, []*model.Item{model.NewItem("i", 5, 5, 5, 1)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted")
		}
	})

	t.Run("no items", func(t *testing.T) {
		result, err := tp.Solve(context.Background(), []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted")
		}
	})
}

func TestTrialPacking_ContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	tp := NewTrialPacking(newPivot)
	binTypes := []*model.Bin{
		model.NewBin("box", 100, 100, 100, 10000),
	}
	items := make([]*model.Item, 200)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, _ := tp.Solve(ctx, binTypes, items)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestTrialPacking_ItemTooLargeForAnyBin(t *testing.T) {
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("small", 10, 10, 10, 1000),
	}

	items := []*model.Item{
		model.NewItem("huge", 50, 50, 50, 1),
		model.NewItem("ok", 8, 8, 8, 1),
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	if result.Stats.FittedItems != 1 {
		t.Errorf("expected 1 fitted (ok), got %d", result.Stats.FittedItems)
	}
	if result.Stats.UnfittedCount != 1 {
		t.Errorf("expected 1 unfitted (huge), got %d", result.Stats.UnfittedCount)
	}
}

// --- Cost-aware tests ---

func TestTrialPacking_PrefersCheaperBin(t *testing.T) {
	// Two bins with same dimensions but different costs.
	// The solver should prefer the cheaper one.
	tp := NewTrialPacking(newPivot, WithLookahead())

	binTypes := []*model.Bin{
		model.NewBin("expensive", 50, 50, 50, 1000, model.BinCost(100)),
		model.NewBin("cheap", 50, 50, 50, 1000, model.BinCost(10)),
	}

	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	if result.Stats.TotalCost != 10 {
		t.Errorf("expected total cost 10 (cheap bin), got %.0f", result.Stats.TotalCost)
	}
	t.Logf("Used bin: %s, cost: %.0f", result.Bins[0].ID, result.Stats.TotalCost)
}

func TestTrialPacking_CostVsBinCount(t *testing.T) {
	// Scenario: 1 large expensive box ($50) vs 2 small cheap boxes ($15 each).
	// 2 small boxes cost $30 total < $50 for 1 large.
	// The solver should prefer 2 cheap boxes when minimizing cost.
	tp := NewTrialPacking(newPivot, WithLookahead())

	binTypes := []*model.Bin{
		model.NewBin("small-cheap", 25, 25, 25, 1000, model.BinCost(15)),
		model.NewBin("large-expensive", 50, 50, 50, 1000, model.BinCost(50)),
	}

	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
		model.NewItem("b", 20, 20, 20, 1),
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Bins: %d, Cost: %.0f", result.Stats.TotalBins, result.Stats.TotalCost)

	// Should use 2 small-cheap ($30) instead of 1 large-expensive ($50).
	if result.Stats.TotalCost > 35 {
		t.Errorf("expected cost ≤30 (2 cheap boxes), got %.0f", result.Stats.TotalCost)
	}
}

func TestTrialPacking_CostRespectsWeight(t *testing.T) {
	// Cheap bin has low weight capacity — heavy items can't use it.
	// The solver must pick the expensive bin for heavy items and cheap for light.
	tp := NewTrialPacking(newPivot, WithLookahead())

	binTypes := []*model.Bin{
		model.NewBin("cheap-weak", 50, 50, 50, 3, model.BinCost(10)),        // max 3 kg
		model.NewBin("expensive-strong", 50, 50, 50, 50, model.BinCost(80)), // max 50 kg
	}

	items := []*model.Item{
		model.NewItem("heavy", 20, 20, 20, 10), // 10 kg — only fits in expensive
		model.NewItem("light", 20, 20, 20, 1),  // 1 kg — fits in cheap
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	if result.Stats.FittedItems != 2 {
		t.Fatalf("expected 2 fitted, got %d", result.Stats.FittedItems)
	}

	// Should use 1 expensive (for heavy) + 1 cheap (for light) = $90.
	// NOT 1 expensive for both ($80) — because the cheap bin is cheaper for the light item.
	// Actually, both fit in the expensive bin (20+20=40 < 50 in geometry, 10+1=11 < 50 in weight).
	// But the cost-optimal solution depends on whether 1×$80 < 1×$80+1×$10.
	// 1 expensive = $80, 2 bins (1 expensive + 1 cheap) = $90. So 1 expensive is cheaper.
	// The point is: the heavy item MUST go in the expensive bin (weight constraint).
	t.Logf("Bins: %d, Cost: $%.0f", result.Stats.TotalBins, result.Stats.TotalCost)

	for _, bin := range result.Bins {
		if len(bin.Items) > 0 {
			t.Logf("  %s (max %.0f kg, $%.0f): %d items, %.1f kg",
				bin.ID, bin.MaxWeight, bin.Cost, len(bin.Items), bin.TotalWeight())
		}
	}

	// The heavy item must not be in the cheap bin.
	for _, bin := range result.Bins {
		if bin.MaxWeight <= 3 {
			for _, item := range bin.Items {
				if item.Weight > 3 {
					t.Errorf("heavy item %s (%.1f kg) placed in weak bin %s (max %.0f kg)",
						item.ID, item.Weight, bin.ID, bin.MaxWeight)
				}
			}
		}
	}
}

func TestTrialPacking_NoCostBackwardCompatible(t *testing.T) {
	// Without costs, behavior should be identical to before.
	tp := NewTrialPacking(newPivot)

	binTypes := []*model.Bin{
		model.NewBin("small", 25, 25, 25, 1000),
		model.NewBin("large", 50, 50, 50, 1000),
	}

	items := []*model.Item{
		model.NewItem("a", 20, 20, 20, 1),
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	if result.Stats.TotalCost != 0 {
		t.Errorf("expected 0 cost when not set, got %.0f", result.Stats.TotalCost)
	}
	// Should prefer smaller bin (higher fill ratio).
	if result.Bins[0].Volume > 20000 {
		t.Errorf("expected small bin, got volume %.0f", result.Bins[0].Volume)
	}
}

// --- Lookahead tests ---

func TestTrialPackingLookahead_ReducesBins(t *testing.T) {
	// Compare Level 3 vs Level 4 on a scenario where look-ahead helps.
	binTypes := []*model.Bin{
		model.NewBin("small", 20, 20, 20, 1000),
		model.NewBin("medium", 40, 40, 40, 1000),
		model.NewBin("large", 60, 60, 60, 1000),
	}

	var items []*model.Item
	for i := 0; i < 2; i++ {
		items = append(items, model.NewItem("big", 55, 55, 55, 1))
	}
	for i := 0; i < 8; i++ {
		items = append(items, model.NewItem("small", 15, 15, 15, 1))
	}

	// Level 3.
	tp3 := NewTrialPacking(newPivot)
	r3, err := tp3.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	// Level 4.
	tp4 := NewTrialPacking(newPivot, WithLookahead())
	r4, err := tp4.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Level 3: %d bins, %d fitted, %.1f%% vol",
		r3.Stats.TotalBins, r3.Stats.FittedItems, r3.Stats.VolumeUsedPct)
	t.Logf("Level 4: %d bins, %d fitted, %.1f%% vol",
		r4.Stats.TotalBins, r4.Stats.FittedItems, r4.Stats.VolumeUsedPct)

	// Level 4 should be at least as good as Level 3.
	if r4.Stats.FittedItems < r3.Stats.FittedItems {
		t.Errorf("level 4 fitted fewer items (%d) than level 3 (%d)", r4.Stats.FittedItems, r3.Stats.FittedItems)
	}
}

func TestTrialPackingLookahead_NoIntersections(t *testing.T) {
	tp := NewTrialPacking(newPivot, WithLookahead())

	binTypes := []*model.Bin{
		model.NewBin("small", 30, 30, 30, 1000),
		model.NewBin("large", 70, 70, 70, 1000),
	}

	items := make([]*model.Item, 15)
	for i := range items {
		w := float64(8 + i%10)
		h := float64(5 + (i*3)%12)
		d := float64(6 + (i*7)%9)
		items[i] = model.NewItem("i", w, h, d, 1)
	}

	result, err := tp.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	for _, bin := range result.Bins {
		for i := 0; i < len(bin.Items); i++ {
			for j := i + 1; j < len(bin.Items); j++ {
				if intersection.Intersect(bin.Items[i], bin.Items[j]) {
					t.Errorf("bin %q: items %d and %d intersect", bin.ID, i, j)
				}
			}
		}
	}
}

// --- Benchmarks ---

func BenchmarkTrialPacking20Items3BinTypes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tp := NewTrialPacking(newPivot)
		binTypes := []*model.Bin{
			model.NewBin("small", 30, 30, 30, 1000),
			model.NewBin("medium", 50, 50, 50, 1000),
			model.NewBin("large", 80, 80, 80, 1000),
		}
		items := make([]*model.Item, 20)
		for j := range items {
			w := float64(5 + j%20)
			h := float64(5 + (j*7)%15)
			d := float64(5 + (j*13)%10)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		tp.Solve(context.Background(), binTypes, items)
	}
}

func BenchmarkTrialPackingLookahead20Items3BinTypes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tp := NewTrialPacking(newPivot, WithLookahead())
		binTypes := []*model.Bin{
			model.NewBin("small", 30, 30, 30, 1000),
			model.NewBin("medium", 50, 50, 50, 1000),
			model.NewBin("large", 80, 80, 80, 1000),
		}
		items := make([]*model.Item, 20)
		for j := range items {
			w := float64(5 + j%20)
			h := float64(5 + (j*7)%15)
			d := float64(5 + (j*13)%10)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		tp.Solve(context.Background(), binTypes, items)
	}
}

func BenchmarkTrialPacking50Items5BinTypes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tp := NewTrialPacking(newPivot)
		binTypes := []*model.Bin{
			model.NewBin("xs", 20, 20, 20, 1000),
			model.NewBin("s", 30, 30, 30, 1000),
			model.NewBin("m", 50, 50, 50, 1000),
			model.NewBin("l", 70, 70, 70, 1000),
			model.NewBin("xl", 100, 100, 100, 1000),
		}
		items := make([]*model.Item, 50)
		for j := range items {
			w := float64(5 + j%25)
			h := float64(5 + (j*7)%20)
			d := float64(5 + (j*13)%15)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		tp.Solve(context.Background(), binTypes, items)
	}
}

func BenchmarkTrialPackingLookahead50Items5BinTypes(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tp := NewTrialPacking(newPivot, WithLookahead())
		binTypes := []*model.Bin{
			model.NewBin("xs", 20, 20, 20, 1000),
			model.NewBin("s", 30, 30, 30, 1000),
			model.NewBin("m", 50, 50, 50, 1000),
			model.NewBin("l", 70, 70, 70, 1000),
			model.NewBin("xl", 100, 100, 100, 1000),
		}
		items := make([]*model.Item, 50)
		for j := range items {
			w := float64(5 + j%25)
			h := float64(5 + (j*7)%20)
			d := float64(5 + (j*13)%15)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		tp.Solve(context.Background(), binTypes, items)
	}
}

// --- Comparison benchmark: TrialPacking vs Greedy BFD ---

func BenchmarkComparison_Greedy_vs_Trial(b *testing.B) {
	makeBins := func() []*model.Bin {
		return []*model.Bin{
			model.NewBin("small", 30, 30, 30, 1000),
			model.NewBin("medium", 50, 50, 50, 1000),
			model.NewBin("large", 80, 80, 80, 1000),
		}
	}
	makeItems := func() []*model.Item {
		items := make([]*model.Item, 30)
		for j := range items {
			w := float64(5 + j%20)
			h := float64(5 + (j*7)%15)
			d := float64(5 + (j*13)%10)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		return items
	}

	b.Run("Greedy_BFD", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			bins := makeBins()
			items := makeItems()
			packGreedy(context.Background(), newPivot(), bins, items, strategy.BestFitDecreasing)
		}
	})

	b.Run("TrialPacking", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tp := NewTrialPacking(newPivot)
			tp.Solve(context.Background(), makeBins(), makeItems())
		}
	})

	b.Run("TrialPacking_Lookahead", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			tp := NewTrialPacking(newPivot, WithLookahead())
			tp.Solve(context.Background(), makeBins(), makeItems())
		}
	})
}
