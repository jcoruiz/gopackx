// Example: Packrift ecommerce carton selection
//
// Demonstrates variable-sized box selection with a compact ecommerce fixture
// based on Packrift's public cartonization solver fixtures:
// https://packrift.github.io/packaging-optimization-benchmark-corpus/cartonization-solver-fixtures.html
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func main() {
	result, err := gopackx.Pack(context.Background(), packriftCartons(), packriftItems())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("=== Packrift Ecommerce Carton Selection ===")
	fmt.Printf("Fitted: %d/%d items\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Cartons used: %d\n", result.Stats.TotalBins)
	fmt.Printf("Total carton cost: $%.2f\n", result.Stats.TotalCost)
	fmt.Println()

	for _, carton := range result.Bins {
		if len(carton.Items) == 0 {
			continue
		}

		fmt.Printf("%s (%gx%gx%g): %d items, %.1f%% volume used\n",
			carton.ID,
			carton.Width,
			carton.Height,
			carton.Depth,
			len(carton.Items),
			carton.VolumeUsedPct(),
		)
	}
}

func packriftCartons() []*model.Bin {
	return []*model.Bin{
		model.NewBin("Packrift 10x6x6 carton", 10, 6, 6, 25, model.BinCost(1.05)),
		model.NewBin("Packrift 16x8x4 carton", 16, 8, 4, 25, model.BinCost(1.35)),
		model.NewBin("Packrift 20x14x6 carton", 20, 14, 6, 35, model.BinCost(1.90)),
		model.NewBin("Packrift 24x10x8 carton", 24, 10, 8, 40, model.BinCost(2.35)),
		model.NewBin("Packrift 40x20x20 carton", 40, 20, 20, 75, model.BinCost(4.95)),
	}
}

func packriftItems() []*model.Item {
	return []*model.Item{
		model.NewItem("demo-small-item", 7.5, 4.5, 3.5, 1),
		model.NewItem("demo-flat-item-1", 15, 7, 2.5, 1.5),
		model.NewItem("demo-flat-item-2", 15, 7, 2.5, 1.5),
		model.NewItem("demo-long-item", 21, 8.5, 5, 3),
		model.NewItem("demo-bulk-item-1", 18, 12, 5.5, 4),
		model.NewItem("demo-bulk-item-2", 18, 12, 5.5, 4),
		model.NewItem("demo-bulk-item-3", 18, 12, 5.5, 4),
		model.NewItem("demo-bulk-item-4", 18, 12, 5.5, 4),
	}
}
