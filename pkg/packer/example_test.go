package packer_test

import (
	"context"
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

func ExamplePacker_Pack() {
	p := packer.NewPacker()
	p.AddBin(model.NewBin("bin-1", 100, 100, 100, 1000))
	p.AddItem(model.NewItem("item-1", 50, 50, 50, 10))
	p.AddItem(model.NewItem("item-2", 40, 40, 40, 8))
	p.AddItem(model.NewItem("item-3", 30, 30, 30, 5))

	result, err := p.Pack(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Bins used: %d\n", result.Stats.TotalBins)
	fmt.Printf("Fitted: %d/%d\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Unfitted: %d\n", result.Stats.UnfittedCount)

	// Output:
	// Bins used: 1
	// Fitted: 3/3
	// Unfitted: 0
}

func ExamplePacker_Pack_withStrategy() {
	p := packer.NewPacker(
		packer.WithStrategy(strategy.MinimizeBins),
	)

	// Two bins available, items should be packed tightly into fewer bins.
	p.AddBin(model.NewBin("bin-1", 60, 60, 60, 500))
	p.AddBin(model.NewBin("bin-2", 60, 60, 60, 500))
	p.AddItem(model.NewItem("a", 30, 30, 30, 5))
	p.AddItem(model.NewItem("b", 30, 30, 30, 5))
	p.AddItem(model.NewItem("c", 30, 30, 30, 5))
	p.AddItem(model.NewItem("d", 30, 30, 30, 5))

	result, err := p.Pack(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Fitted: %d/%d\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Unfitted: %d\n", result.Stats.UnfittedCount)

	// Output:
	// Fitted: 4/4
	// Unfitted: 0
}

func ExamplePacker_Pack_withEngine() {
	p := packer.NewPacker(
		packer.WithPlacementEngine(placement.NewExtremePointEngine()),
	)

	p.AddBin(model.NewBin("bin-1", 100, 100, 100, 1000))
	p.AddItem(model.NewItem("item-1", 50, 50, 50, 10))
	p.AddItem(model.NewItem("item-2", 50, 50, 50, 10))
	p.AddItem(model.NewItem("item-3", 50, 50, 50, 10))

	result, err := p.Pack(context.Background())
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Fitted: %d/%d\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Unfitted: %d\n", result.Stats.UnfittedCount)

	// Output:
	// Fitted: 3/3
	// Unfitted: 0
}
