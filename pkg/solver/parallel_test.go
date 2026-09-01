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

func TestParallelSolve(t *testing.T) {
	p := NewParallel()

	bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
	items := make([]*model.Item, 10)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, err := p.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems == 0 {
		t.Error("expected at least some items")
	}
}

func TestParallelNoIntersections(t *testing.T) {
	p := NewParallel()

	bins := []*model.Bin{model.NewBin("box", 50, 50, 50, 1000)}
	items := make([]*model.Item, 20)
	for i := range items {
		w := float64(5 + i%10)
		h := float64(3 + (i*3)%8)
		d := float64(4 + (i*7)%9)
		items[i] = model.NewItem("i", w, h, d, 1)
	}

	result, err := p.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}

	for _, bin := range result.Bins {
		for i := range len(bin.Items) {
			for j := i + 1; j < len(bin.Items); j++ {
				if intersection.Intersect(bin.Items[i], bin.Items[j]) {
					t.Errorf("items %d and %d intersect", i, j)
				}
			}
		}
	}
}

func TestParallelBetterOrEqualToSingle(t *testing.T) {
	p := NewParallel()

	bins := []*model.Bin{model.NewBin("box", 40, 35, 40, 10000)}
	items := make([]*model.Item, 30)
	for i := range items {
		w := float64(8 + i%12)
		h := float64(6 + (i*7)%10)
		d := float64(7 + (i*13)%11)
		items[i] = model.NewItem("i", w, h, d, 1)
	}

	result, err := p.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}

	// Single engine result for comparison.
	engine := placement.NewPivotEngine()
	singleBin := model.NewBin("box", 40, 35, 40, 10000)
	for _, item := range items {
		ic := model.NewItem(item.ID, item.Width, item.Height, item.Depth, item.Weight)
		engine.PlaceItem(singleBin, ic)
	}

	t.Logf("parallel=%d, single-pivot=%d", result.Stats.FittedItems, len(singleBin.Items))
	if result.Stats.FittedItems < len(singleBin.Items) {
		t.Errorf("parallel (%d) should be >= single pivot (%d)", result.Stats.FittedItems, len(singleBin.Items))
	}
}

func TestParallelDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	p := NewParallel()
	bins := []*model.Bin{model.NewBin("box", 100, 100, 100, 10000)}
	items := make([]*model.Item, 50)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, err := p.Solve(ctx, bins, items)
	if err != nil {
		t.Logf("got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
}

func TestParallelEmptyInputs(t *testing.T) {
	p := NewParallel()

	t.Run("no bins", func(t *testing.T) {
		result, err := p.Solve(context.Background(), nil, []*model.Item{model.NewItem("i", 5, 5, 5, 1)})
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted items")
		}
	})

	t.Run("no items", func(t *testing.T) {
		result, err := p.Solve(context.Background(), []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		if result.Stats.FittedItems != 0 {
			t.Error("expected 0 fitted items")
		}
	})
}

func TestParallelCustomConfigs(t *testing.T) {
	p := NewParallel(
		WithConfig(func() placement.Engine { return placement.NewPivotEngine() }, strategy.BestFit),
		WithConfig(func() placement.Engine { return placement.NewLAFFEngine() }, strategy.MinimizeBins),
	)

	bins := []*model.Bin{model.NewBin("box", 30, 30, 30, 1000)}
	items := make([]*model.Item, 5)
	for i := range items {
		items[i] = model.NewItem("i", 10, 10, 10, 1)
	}

	result, err := p.Solve(context.Background(), bins, items)
	if err != nil {
		t.Fatal(err)
	}
	if result.Stats.FittedItems == 0 {
		t.Error("expected at least some items")
	}
}

func BenchmarkParallel50Items(b *testing.B) {
	for range b.N {
		p := NewParallel()
		bins := []*model.Bin{
			model.NewBin("b1", 100, 100, 100, 10000),
			model.NewBin("b2", 100, 100, 100, 10000),
		}
		items := make([]*model.Item, 50)
		for j := range items {
			w := float64(5 + j%20)
			h := float64(5 + (j*7)%15)
			d := float64(5 + (j*13)%10)
			items[j] = model.NewItem("i", w, h, d, 1)
		}
		_, _ = p.Solve(context.Background(), bins, items)
	}
}
