// Example: placement engine comparison
//
// Compares the four placement engines (Pivot, ExtremePoint, LAFF, LAFF-Fast)
// on the same set of items and bins. Prints fitted items, volume used,
// and time taken for each engine.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
)

// engineInfo pairs an engine constructor with its name.
type engineInfo struct {
	name      string
	newEngine func() placement.Engine
}

func main() {
	fmt.Println("=== Placement Engine Comparison ===")
	fmt.Println()
	fmt.Println("Same items and bins are used for each engine.")
	fmt.Println()

	engines := []engineInfo{
		{"PivotEngine", func() placement.Engine { return placement.NewPivotEngine() }},
		{"ExtremePointEngine", func() placement.Engine { return placement.NewExtremePointEngine() }},
		{"LAFFEngine", func() placement.Engine { return placement.NewLAFFEngine() }},
		{"LAFFEngine (Fast)", func() placement.Engine { return placement.NewLAFFEngine(placement.LAFFFast()) }},
	}

	// Print header.
	fmt.Printf("%-22s  %7s  %7s  %9s  %9s  %12s\n",
		"Engine", "Fitted", "Unfit", "Vol Used%", "Wt Used%", "Time")
	fmt.Printf("%-22s  %7s  %7s  %9s  %9s  %12s\n",
		"----------------------", "-------", "-------", "---------", "---------", "------------")

	for _, ei := range engines {
		result, elapsed := runWithEngine(ei.newEngine())
		fmt.Printf("%-22s  %7d  %7d  %8.1f%%  %8.1f%%  %12s\n",
			ei.name,
			result.Stats.FittedItems,
			result.Stats.UnfittedCount,
			result.Stats.VolumeUsedPct,
			result.Stats.WeightUsedPct,
			elapsed.Round(time.Microsecond))
	}

	fmt.Println()
	fmt.Println("Engine descriptions:")
	fmt.Println("  PivotEngine        - Generates pivot points from corners of placed items")
	fmt.Println("  ExtremePointEngine  - Maintains candidate positions with space metadata for fast rejection")
	fmt.Println("  LAFFEngine          - Largest Area Fit First; divides bin into horizontal levels")
	fmt.Println("  LAFFEngine (Fast)   - LAFF variant with 2D-only placement within levels (faster)")
}

// runWithEngine creates fresh bins/items and packs them using the given engine.
func runWithEngine(engine placement.Engine) (*model.Result, time.Duration) {
	p := packer.NewPacker(packer.WithPlacementEngine(engine))

	// Two bins.
	p.AddBin(model.NewBin("bin-A", 60, 60, 60, 200))
	p.AddBin(model.NewBin("bin-B", 80, 80, 80, 400))

	// Items of various sizes.
	items := []struct {
		id          string
		w, h, d, wt float64
	}{
		{"box-01", 30, 25, 20, 10},
		{"box-02", 35, 30, 25, 15},
		{"box-03", 20, 15, 10, 5},
		{"box-04", 25, 20, 15, 8},
		{"box-05", 40, 35, 30, 20},
		{"box-06", 15, 10, 10, 3},
		{"box-07", 50, 40, 35, 25},
		{"box-08", 20, 20, 20, 7},
		{"box-09", 30, 30, 25, 12},
		{"box-10", 35, 25, 20, 14},
		{"box-11", 10, 10, 10, 2},
		{"box-12", 45, 35, 30, 18},
		{"box-13", 25, 15, 15, 6},
		{"box-14", 20, 20, 15, 9},
		{"box-15", 30, 20, 20, 11},
	}
	for _, it := range items {
		p.AddItem(model.NewItem(it.id, it.w, it.h, it.d, it.wt))
	}

	start := time.Now()
	result, err := p.Pack(context.Background())
	elapsed := time.Since(start)

	if err != nil {
		log.Fatalf("engine failed: %v", err)
	}
	return result, elapsed
}
