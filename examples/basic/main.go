// Example: basic bin packing
//
// Creates a single bin, adds several items of different sizes, packs them,
// and prints the results: fitted count, volume utilization, and item positions.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
)

func main() {
	// Create a packer with default settings (BestFitDecreasing strategy, PivotEngine).
	p := packer.NewPacker()

	// Add a bin: 100x100x100 cm, max weight 500 kg.
	bin := model.NewBin("bin-1", 100, 100, 100, 500)
	p.AddBin(bin)

	// Add items of various sizes and weights.
	items := []*model.Item{
		model.NewItem("box-A", 50, 50, 50, 20),
		model.NewItem("box-B", 40, 30, 30, 15),
		model.NewItem("box-C", 30, 20, 20, 10),
		model.NewItem("box-D", 25, 25, 25, 12),
		model.NewItem("box-E", 60, 40, 40, 30),
		model.NewItem("box-F", 20, 20, 20, 5),
		model.NewItem("box-G", 35, 25, 15, 8),
	}
	for _, item := range items {
		p.AddItem(item)
	}

	// Pack the items.
	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatalf("packing failed: %v", err)
	}

	// Print summary statistics.
	fmt.Println("=== Basic Bin Packing ===")
	fmt.Println()
	fmt.Printf("Total items:    %d\n", result.Stats.TotalItems)
	fmt.Printf("Fitted items:   %d\n", result.Stats.FittedItems)
	fmt.Printf("Unfitted items: %d\n", result.Stats.UnfittedCount)
	fmt.Printf("Volume used:    %.1f%%\n", result.Stats.VolumeUsedPct)
	fmt.Printf("Weight used:    %.1f%%\n", result.Stats.WeightUsedPct)
	fmt.Println()

	// Print each fitted item's placement details.
	for _, b := range result.Bins {
		if len(b.Items) == 0 {
			continue
		}
		fmt.Printf("Bin %q (%gx%gx%g, max %.0f kg)\n", b.ID, b.Width, b.Height, b.Depth, b.MaxWeight)
		fmt.Printf("  Items packed: %d | Volume: %.1f%% | Weight: %.1f / %.1f kg\n",
			len(b.Items), b.VolumeUsedPct(), b.TotalWeight(), b.MaxWeight)
		fmt.Println()
		fmt.Printf("  %-10s  %-18s  %-22s  %s\n", "ID", "Size (WxHxD)", "Position (X, Y, Z)", "Rotation")
		fmt.Printf("  %-10s  %-18s  %-22s  %s\n", "----------", "------------------", "----------------------", "--------")
		for _, item := range b.Items {
			dim := item.Dimension()
			fmt.Printf("  %-10s  %5.0f x %5.0f x %5.0f   (%6.1f, %6.1f, %6.1f)   %d\n",
				item.ID, dim[0], dim[1], dim[2],
				item.Position[0], item.Position[1], item.Position[2],
				item.RotationType)
		}
	}

	// Print unfitted items if any.
	if len(result.UnfittedItems) > 0 {
		fmt.Println()
		fmt.Println("Unfitted items:")
		for _, item := range result.UnfittedItems {
			fmt.Printf("  %s (%gx%gx%g, %.1f kg)\n", item.ID, item.Width, item.Height, item.Depth, item.Weight)
		}
	}
}
