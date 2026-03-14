// Example: multiple containers
//
// Distributes many items across several bins of different sizes.
// Shows which items go into which bin and prints per-bin statistics.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
)

func main() {
	fmt.Println("=== Multi-Bin Packing ===")
	fmt.Println()

	p := packer.NewPacker()

	// Three bins of different sizes.
	bins := []*model.Bin{
		model.NewBin("small-box", 40, 40, 40, 100),
		model.NewBin("medium-box", 60, 60, 60, 200),
		model.NewBin("large-box", 80, 80, 80, 400),
	}
	for _, b := range bins {
		p.AddBin(b)
	}

	// Fifteen items that need to be distributed.
	items := []*model.Item{
		model.NewItem("item-01", 30, 25, 20, 10),
		model.NewItem("item-02", 35, 30, 25, 15),
		model.NewItem("item-03", 20, 15, 10, 5),
		model.NewItem("item-04", 25, 20, 15, 8),
		model.NewItem("item-05", 40, 35, 30, 20),
		model.NewItem("item-06", 15, 10, 10, 3),
		model.NewItem("item-07", 50, 40, 35, 25),
		model.NewItem("item-08", 20, 20, 20, 7),
		model.NewItem("item-09", 30, 30, 25, 12),
		model.NewItem("item-10", 35, 25, 20, 14),
		model.NewItem("item-11", 10, 10, 10, 2),
		model.NewItem("item-12", 45, 35, 30, 18),
		model.NewItem("item-13", 25, 15, 15, 6),
		model.NewItem("item-14", 20, 20, 15, 9),
		model.NewItem("item-15", 30, 20, 20, 11),
	}
	for _, item := range items {
		p.AddItem(item)
	}

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatalf("packing failed: %v", err)
	}

	// Print overall summary.
	fmt.Printf("Total items:    %d\n", result.Stats.TotalItems)
	fmt.Printf("Fitted items:   %d\n", result.Stats.FittedItems)
	fmt.Printf("Unfitted items: %d\n", result.Stats.UnfittedCount)
	fmt.Printf("Bins used:      %d / %d\n", result.Stats.TotalBins, len(bins))
	fmt.Printf("Avg volume used: %.1f%%\n", result.Stats.VolumeUsedPct)
	fmt.Println()

	// Print per-bin details.
	for _, b := range result.Bins {
		fmt.Printf("Bin %q (%gx%gx%g, max %.0f kg)\n", b.ID, b.Width, b.Height, b.Depth, b.MaxWeight)
		if len(b.Items) == 0 {
			fmt.Println("  (empty)")
			fmt.Println()
			continue
		}
		fmt.Printf("  Items:  %d\n", len(b.Items))
		fmt.Printf("  Volume: %.1f%% (%.0f / %.0f)\n", b.VolumeUsedPct(), b.UsedVolume(), b.Volume)
		fmt.Printf("  Weight: %.1f / %.1f kg (remaining: %.1f kg)\n",
			b.TotalWeight(), b.MaxWeight, b.RemainingWeight())
		fmt.Println()
		fmt.Printf("  %-10s  %-18s  %-22s\n", "ID", "Size (WxHxD)", "Position (X, Y, Z)")
		fmt.Printf("  %-10s  %-18s  %-22s\n", "----------", "------------------", "----------------------")
		for _, item := range b.Items {
			dim := item.Dimension()
			fmt.Printf("  %-10s  %5.0f x %5.0f x %5.0f   (%6.1f, %6.1f, %6.1f)\n",
				item.ID, dim[0], dim[1], dim[2],
				item.Position[0], item.Position[1], item.Position[2])
		}
		fmt.Println()
	}

	// Print unfitted items if any.
	if len(result.UnfittedItems) > 0 {
		fmt.Println("Unfitted items (did not fit in any bin):")
		for _, item := range result.UnfittedItems {
			fmt.Printf("  %s (%gx%gx%g, %.1f kg, vol=%.0f)\n",
				item.ID, item.Width, item.Height, item.Depth, item.Weight, item.Volume)
		}
	}
}
