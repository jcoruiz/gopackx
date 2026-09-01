package packer

import (
	"context"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

func TestPackBasic(t *testing.T) {
	p := NewPacker()
	p.AddBin(model.NewBin("box", 20, 20, 20, 100))
	p.AddItem(model.NewItem("i1", 10, 10, 10, 5))
	p.AddItem(model.NewItem("i2", 10, 10, 10, 5))

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("FittedItems = %d, want 2", result.Stats.FittedItems)
	}
	if len(result.UnfittedItems) != 0 {
		t.Errorf("UnfittedItems = %d, want 0", len(result.UnfittedItems))
	}
}

func TestPackUnfitted(t *testing.T) {
	p := NewPacker()
	p.AddBin(model.NewBin("tiny", 5, 5, 5, 100))
	p.AddItem(model.NewItem("big", 10, 10, 10, 1))

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if result.Stats.FittedItems != 0 {
		t.Errorf("FittedItems = %d, want 0", result.Stats.FittedItems)
	}
	if len(result.UnfittedItems) != 1 {
		t.Errorf("UnfittedItems = %d, want 1", len(result.UnfittedItems))
	}
}

func TestPackMultipleBins(t *testing.T) {
	p := NewPacker(WithStrategy(strategy.MinimizeBins))
	p.AddBin(model.NewBin("b1", 10, 10, 10, 100))
	p.AddBin(model.NewBin("b2", 10, 10, 10, 100))

	// Two items that can't fit in one 10x10x10 bin.
	p.AddItem(model.NewItem("i1", 10, 10, 10, 1))
	p.AddItem(model.NewItem("i2", 10, 10, 10, 1))

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("FittedItems = %d, want 2", result.Stats.FittedItems)
	}
	if result.Stats.TotalBins != 2 {
		t.Errorf("TotalBins = %d, want 2", result.Stats.TotalBins)
	}
}

func TestPackWithStability(t *testing.T) {
	engine := placement.NewPivotEngine(placement.WithStability(0.75))
	p := NewPacker(WithPlacementEngine(engine))
	p.AddBin(model.NewBin("box", 20, 20, 20, 100))
	p.AddItem(model.NewItem("base", 10, 10, 10, 5))
	p.AddItem(model.NewItem("top", 10, 10, 10, 3))

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("FittedItems = %d, want 2", result.Stats.FittedItems)
	}
}

func TestPackNoIntersections(t *testing.T) {
	p := NewPacker()
	p.AddBin(model.NewBin("box", 50, 50, 50, 1000))

	for range 20 {
		p.AddItem(model.NewItem("i", 10, 10, 10, 1))
	}

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}

	for _, bin := range result.Bins {
		for i := range bin.Items {
			for j := i + 1; j < len(bin.Items); j++ {
				if intersection.Intersect(bin.Items[i], bin.Items[j]) {
					t.Errorf("items %d and %d in bin %s intersect", i, j, bin.ID)
				}
			}
		}
	}
}

func TestPackContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	p := NewPacker()
	p.AddBin(model.NewBin("box", 100, 100, 100, 1000))
	p.AddItem(model.NewItem("i1", 10, 10, 10, 1))

	_, err := p.Pack(ctx)
	if err == nil {
		t.Error("expected error from cancelled context")
	}
}

func TestPackEmptyInputs(t *testing.T) {
	t.Run("no bins", func(t *testing.T) {
		p := NewPacker()
		p.AddItem(model.NewItem("i1", 10, 10, 10, 1))

		result, err := p.Pack(context.Background())
		if err != nil {
			t.Fatalf("Pack() error: %v", err)
		}
		if len(result.UnfittedItems) != 1 {
			t.Errorf("UnfittedItems = %d, want 1", len(result.UnfittedItems))
		}
	})

	t.Run("no items", func(t *testing.T) {
		p := NewPacker()
		p.AddBin(model.NewBin("box", 10, 10, 10, 100))

		result, err := p.Pack(context.Background())
		if err != nil {
			t.Fatalf("Pack() error: %v", err)
		}
		if result.Stats.FittedItems != 0 {
			t.Errorf("FittedItems = %d, want 0", result.Stats.FittedItems)
		}
	})
}

func TestPackNextFitStrategy(t *testing.T) {
	p := NewPacker(WithStrategy(strategy.NextFit))
	p.AddBin(model.NewBin("b1", 10, 10, 10, 100))
	p.AddBin(model.NewBin("b2", 10, 10, 10, 100))

	p.AddItem(model.NewItem("i1", 10, 10, 10, 1))
	p.AddItem(model.NewItem("i2", 10, 10, 10, 1))

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if result.Stats.FittedItems != 2 {
		t.Errorf("FittedItems = %d, want 2", result.Stats.FittedItems)
	}
}

func TestPackAllStrategies(t *testing.T) {
	strategies := []struct {
		name string
		st   strategy.Type
	}{
		{"MinimizeBins", strategy.MinimizeBins},
		{"Greedy", strategy.Greedy},
		{"BestFit", strategy.BestFit},
		{"BestFitDecreasing", strategy.BestFitDecreasing},
		{"NextFit", strategy.NextFit},
		{"WorstFit", strategy.WorstFit},
		{"AlmostWorstFit", strategy.AlmostWorstFit},
	}

	for _, tt := range strategies {
		t.Run(tt.name, func(t *testing.T) {
			p := NewPacker(WithStrategy(tt.st))
			p.AddBin(model.NewBin("box", 30, 30, 30, 1000))

			p.AddItem(model.NewItem("large", 20, 20, 20, 1))
			p.AddItem(model.NewItem("small1", 10, 10, 10, 1))
			p.AddItem(model.NewItem("small2", 10, 10, 10, 1))

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatalf("Pack() error: %v", err)
			}
			if result.Stats.FittedItems == 0 {
				t.Error("expected at least 1 fitted item")
			}
		})
	}
}

func TestPackStats(t *testing.T) {
	p := NewPacker()
	p.AddBin(model.NewBin("box", 10, 10, 10, 100))
	p.AddItem(model.NewItem("i1", 10, 10, 10, 50))

	result, err := p.Pack(context.Background())
	if err != nil {
		t.Fatalf("Pack() error: %v", err)
	}
	if result.Stats.VolumeUsedPct != 100 {
		t.Errorf("VolumeUsedPct = %f, want 100", result.Stats.VolumeUsedPct)
	}
	if result.Stats.WeightUsedPct != 50 {
		t.Errorf("WeightUsedPct = %f, want 50", result.Stats.WeightUsedPct)
	}
}

func BenchmarkPack50Items_Pivot(b *testing.B) {
	benchmarkPack50(b, placement.NewPivotEngine())
}

func BenchmarkPack50Items_ExtremePoints(b *testing.B) {
	benchmarkPack50(b, placement.NewExtremePointEngine())
}

func BenchmarkPack50Items_LAFF(b *testing.B) {
	benchmarkPack50(b, placement.NewLAFFEngine())
}

func BenchmarkPack50Items_LAFFFast(b *testing.B) {
	benchmarkPack50(b, placement.NewLAFFEngine(placement.LAFFFast()))
}

func benchmarkPack50(b *testing.B, engine placement.Engine) {
	b.Helper()
	for range b.N {
		p := NewPacker(WithPlacementEngine(engine))
		p.AddBin(model.NewBin("b1", 100, 100, 100, 10000))
		p.AddBin(model.NewBin("b2", 100, 100, 100, 10000))
		p.AddBin(model.NewBin("b3", 100, 100, 100, 10000))

		for j := range 50 {
			w := float64(5 + j%20)
			h := float64(5 + (j*7)%15)
			d := float64(5 + (j*13)%10)
			p.AddItem(model.NewItem("i", w, h, d, 1))
		}

		_, _ = p.Pack(context.Background())
	}
}

func TestPackingQualityComparison(t *testing.T) {
	makeItems := func() []*model.Item {
		items := make([]*model.Item, 40)
		for i := range items {
			w := float64(8 + i%12)
			h := float64(6 + (i*7)%10)
			d := float64(7 + (i*13)%11)
			items[i] = model.NewItem("i", w, h, d, 1)
		}
		return items
	}

	engines := []struct {
		name   string
		engine placement.Engine
	}{
		{"Pivot", placement.NewPivotEngine()},
		{"ExtremePoints", placement.NewExtremePointEngine()},
		{"LAFF", placement.NewLAFFEngine()},
		{"LAFF-Fast", placement.NewLAFFEngine(placement.LAFFFast())},
	}

	for _, tt := range engines {
		t.Run(tt.name, func(t *testing.T) {
			// Single tight bin to force competition for space.
			p := NewPacker(WithPlacementEngine(tt.engine))
			p.AddBin(model.NewBin("b1", 40, 35, 40, 10000))

			for _, item := range makeItems() {
				p.AddItem(item)
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatalf("Pack() error: %v", err)
			}

			t.Logf("fitted=%d/%d bins=%d volume=%.1f%%",
				result.Stats.FittedItems, result.Stats.TotalItems,
				result.Stats.TotalBins, result.Stats.VolumeUsedPct)

			// All engines should place at least some items.
			if result.Stats.FittedItems == 0 {
				t.Error("expected at least 1 fitted item")
			}

			// Verify no intersections.
			for _, bin := range result.Bins {
				for i := range bin.Items {
					for j := i + 1; j < len(bin.Items); j++ {
						if intersection.Intersect(bin.Items[i], bin.Items[j]) {
							t.Errorf("items %d and %d in bin %s intersect", i, j, bin.ID)
						}
					}
				}
			}
		})
	}
}
