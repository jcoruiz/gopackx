package gopackx_test

import (
	"context"
	"math"
	"testing"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestPackPackriftEcommerceCartonFixture(t *testing.T) {
	// Fixture source:
	// https://packrift.github.io/packaging-optimization-benchmark-corpus/cartonization-solver-fixtures.html
	// Dimensions are in inches and weights are in pounds.
	boxTypes := []*model.Bin{
		model.NewBin("Packrift 10x6x6 carton", 10, 6, 6, 25, model.BinCost(1.05)),
		model.NewBin("Packrift 16x8x4 carton", 16, 8, 4, 25, model.BinCost(1.35)),
		model.NewBin("Packrift 20x14x6 carton", 20, 14, 6, 35, model.BinCost(1.90)),
		model.NewBin("Packrift 24x10x8 carton", 24, 10, 8, 40, model.BinCost(2.35)),
		model.NewBin("Packrift 40x20x20 carton", 40, 20, 20, 75, model.BinCost(4.95)),
	}
	items := []*model.Item{
		model.NewItem("demo-small-item", 7.5, 4.5, 3.5, 1),
		model.NewItem("demo-flat-item-1", 15, 7, 2.5, 1.5),
		model.NewItem("demo-flat-item-2", 15, 7, 2.5, 1.5),
		model.NewItem("demo-long-item", 21, 8.5, 5, 3),
		model.NewItem("demo-bulk-item-1", 18, 12, 5.5, 4),
		model.NewItem("demo-bulk-item-2", 18, 12, 5.5, 4),
		model.NewItem("demo-bulk-item-3", 18, 12, 5.5, 4),
		model.NewItem("demo-bulk-item-4", 18, 12, 5.5, 4),
	}

	result, err := gopackx.Pack(context.Background(), boxTypes, items)
	if err != nil {
		t.Fatalf("Pack returned error: %v", err)
	}

	if result.Stats.FittedItems != len(items) {
		t.Fatalf("Fitted %d items, want %d", result.Stats.FittedItems, len(items))
	}
	if len(result.UnfittedItems) != 0 {
		t.Fatalf("Got unfitted items: %v", result.UnfittedItems)
	}
	if result.Stats.TotalBins != 7 {
		t.Fatalf("Used %d cartons, want 7 cost-minimized cartons", result.Stats.TotalBins)
	}
	if math.Abs(result.Stats.TotalCost-12.35) > 0.001 {
		t.Fatalf("Total carton cost = %.2f, want 12.35", result.Stats.TotalCost)
	}
}
