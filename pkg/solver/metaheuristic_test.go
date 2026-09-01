package solver

import (
	"context"
	"testing"
	"time"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
)

// order9904Items returns the 17 items from a real office-supply order.
func order9904Items() []*model.Item {
	return []*model.Item{
		model.NewItem("3720248-1", 27, 8.5, 35.2, 0.395),
		model.NewItem("3720248-2", 27, 8.5, 35.2, 0.395),
		model.NewItem("3720248-3", 27, 8.5, 35.2, 0.395),
		model.NewItem("6500742-1", 27, 9, 30.5, 0.415),
		model.NewItem("6500742-2", 27, 9, 30.5, 0.415),
		model.NewItem("6500742-3", 27, 9, 30.5, 0.415),
		model.NewItem("6500742-4", 27, 9, 30.5, 0.415),
		model.NewItem("6500742-5", 27, 9, 30.5, 0.415),
		model.NewItem("10270023-1", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("10270023-2", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("10270023-3", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("10270023-4", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("2750283-1", 23.5, 6, 34.5, 2.780),
		model.NewItem("2750283-2", 23.5, 6, 34.5, 2.780),
		model.NewItem("2750283-3", 23.5, 6, 34.5, 2.780),
		model.NewItem("P0001234-1", 28, 5, 22, 2.500),
		model.NewItem("P0001234-2", 28, 5, 22, 2.500),
	}
}

// order9904BinTypes returns the 4 available box types.
func order9904BinTypes() []*model.Bin {
	return []*model.Bin{
		model.NewBin("BOLSA-01", 43, 34.5, 5, 1),
		model.NewBin("CAJA-01", 25, 20, 20, 5),
		model.NewBin("CAJA-02", 30, 30, 25, 10),
		model.NewBin("CAJA-04", 40, 40, 30, 20),
	}
}

func TestMetaheuristic_Order9904(t *testing.T) {
	engines := []struct {
		name string
		fn   func() placement.Engine
	}{
		{"Pivot", func() placement.Engine { return placement.NewPivotEngine() }},
		{"ExtremePoint", func() placement.Engine { return placement.NewExtremePointEngine() }},
		{"MaxRects", func() placement.Engine { return placement.NewMaxRectsEngine() }},
	}

	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			m := NewMetaheuristic(eng.fn, MetaMaxIter(2000), MetaMaxNoImprove(500))

			result, err := m.Solve(context.Background(), order9904BinTypes(), order9904Items())
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("Metaheuristic (%s): Fitted %d/17, Bins: %d, Vol: %.1f%%",
				eng.name, result.Stats.FittedItems, result.Stats.TotalBins, result.Stats.VolumeUsedPct)

			for _, bin := range result.Bins {
				if len(bin.Items) == 0 {
					continue
				}
				t.Logf("  %s (%gx%gx%g): %d items, %.1f%% vol, %.2f/%.0f kg",
					bin.ID, bin.Width, bin.Height, bin.Depth,
					len(bin.Items), bin.VolumeUsedPct(), bin.TotalWeight(), bin.MaxWeight)
				for _, item := range bin.Items {
					dim := item.Dimension()
					t.Logf("    %-12s %5.1f×%4.1f×%4.1f pos(%5.1f,%5.1f,%5.1f)",
						item.ID, dim[0], dim[1], dim[2],
						item.Position[0], item.Position[1], item.Position[2])
				}
			}

			if result.Stats.UnfittedCount > 0 {
				t.Logf("  Unfitted: %d", result.Stats.UnfittedCount)
				for _, item := range result.UnfittedItems {
					t.Logf("    %s (%gx%gx%g)", item.ID, item.Width, item.Height, item.Depth)
				}
			}

			// Verify no intersections.
			for _, bin := range result.Bins {
				for i := range len(bin.Items) {
					for j := i + 1; j < len(bin.Items); j++ {
						if intersection.Intersect(bin.Items[i], bin.Items[j]) {
							t.Errorf("INTERSECTION in %s: %s and %s", bin.ID, bin.Items[i].ID, bin.Items[j].ID)
						}
					}
				}
			}

			if result.Stats.FittedItems != 17 {
				t.Errorf("expected all 17 items fitted, got %d", result.Stats.FittedItems)
			}
		})
	}
}

func TestMetaheuristic_SmallItemsTail(t *testing.T) {
	m := NewMetaheuristic(func() placement.Engine { return placement.NewPivotEngine() })

	binTypes := []*model.Bin{
		model.NewBin("small", 20, 20, 20, 1000),
		model.NewBin("medium", 40, 40, 40, 1000),
		model.NewBin("large", 60, 60, 60, 1000),
	}

	var items []*model.Item
	for range 2 {
		items = append(items, model.NewItem("big", 55, 55, 55, 1))
	}
	for range 8 {
		items = append(items, model.NewItem("small", 15, 15, 15, 1))
	}

	result, err := m.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Fitted: %d/10, Bins: %d, Vol: %.1f%%",
		result.Stats.FittedItems, result.Stats.TotalBins, result.Stats.VolumeUsedPct)

	if result.Stats.FittedItems != 10 {
		t.Errorf("expected 10 fitted, got %d", result.Stats.FittedItems)
	}
	if result.Stats.TotalBins > 3 {
		t.Errorf("expected ≤3 bins, got %d", result.Stats.TotalBins)
	}
}

func TestMetaheuristic_NoIntersections(t *testing.T) {
	m := NewMetaheuristic(func() placement.Engine { return placement.NewPivotEngine() })

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

	result, err := m.Solve(context.Background(), binTypes, items)
	if err != nil {
		t.Fatal(err)
	}

	for _, bin := range result.Bins {
		for i := range len(bin.Items) {
			for j := i + 1; j < len(bin.Items); j++ {
				if intersection.Intersect(bin.Items[i], bin.Items[j]) {
					t.Errorf("items %d and %d intersect in bin %s", i, j, bin.ID)
				}
			}
		}
	}
}

func TestMetaheuristic_EmptyInputs(t *testing.T) {
	m := NewMetaheuristic(func() placement.Engine { return placement.NewPivotEngine() })

	t.Run("no bins", func(t *testing.T) {
		result, err := m.Solve(context.Background(), nil, []*model.Item{model.NewItem("i", 5, 5, 5, 1)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted")
		}
	})

	t.Run("no items", func(t *testing.T) {
		result, err := m.Solve(context.Background(), []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted")
		}
	})
}

func TestMetaheuristic_ContextDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	m := NewMetaheuristic(func() placement.Engine { return placement.NewPivotEngine() })
	binTypes := []*model.Bin{
		model.NewBin("box", 50, 50, 50, 1000),
	}
	items := make([]*model.Item, 30)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, _ := m.Solve(ctx, binTypes, items)
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestMetaheuristic_AtLeastAsGoodAsSeed(t *testing.T) {
	eng := func() placement.Engine { return placement.NewPivotEngine() }

	binTypes := []*model.Bin{
		model.NewBin("small", 30, 30, 30, 1000),
		model.NewBin("medium", 50, 50, 50, 1000),
		model.NewBin("large", 80, 80, 80, 1000),
	}

	items := make([]*model.Item, 25)
	for i := range items {
		w := float64(8 + i%15)
		h := float64(5 + (i*3)%12)
		d := float64(6 + (i*7)%10)
		items[i] = model.NewItem("i", w, h, d, 1)
	}

	// Seed.
	seed := NewTrialPacking(eng, WithLookahead())
	seedResult, _ := seed.Solve(context.Background(), binTypes, items)

	// Metaheuristic.
	meta := NewMetaheuristic(eng)
	metaResult, _ := meta.Solve(context.Background(), binTypes, items)

	t.Logf("Seed: %d fitted, %d bins | Meta: %d fitted, %d bins",
		seedResult.Stats.FittedItems, seedResult.Stats.TotalBins,
		metaResult.Stats.FittedItems, metaResult.Stats.TotalBins)

	if metaResult.Stats.FittedItems < seedResult.Stats.FittedItems {
		t.Errorf("metaheuristic (%d fitted) should be >= seed (%d fitted)",
			metaResult.Stats.FittedItems, seedResult.Stats.FittedItems)
	}
}

// --- Benchmarks ---

func BenchmarkMetaheuristic_Order9904(b *testing.B) {
	for range b.N {
		m := NewMetaheuristic(func() placement.Engine { return placement.NewPivotEngine() })
		_, _ = m.Solve(context.Background(), order9904BinTypes(), order9904Items())
	}
}

func BenchmarkMetaheuristic_20Items3BinTypes(b *testing.B) {
	for range b.N {
		m := NewMetaheuristic(func() placement.Engine { return placement.NewPivotEngine() })
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
		_, _ = m.Solve(context.Background(), binTypes, items)
	}
}
