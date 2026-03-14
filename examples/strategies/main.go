// Example: strategy comparison
//
// Compares all 7 packing strategies on the same set of items and bins.
// Prints a comparison table showing how strategy choice affects the result.
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// strategyInfo pairs a strategy constant with its name.
type strategyInfo struct {
	typ  strategy.Type
	name string
}

var strategies = []strategyInfo{
	{strategy.MinimizeBins, "MinimizeBins"},
	{strategy.Greedy, "Greedy"},
	{strategy.BestFit, "BestFit"},
	{strategy.BestFitDecreasing, "BestFitDecreasing"},
	{strategy.NextFit, "NextFit"},
	{strategy.WorstFit, "WorstFit"},
	{strategy.AlmostWorstFit, "AlmostWorstFit"},
}

func main() {
	fmt.Println("=== Strategy Comparison ===")
	fmt.Println()
	fmt.Println("Same items and bins are used for each strategy.")
	fmt.Println()

	// Print header.
	fmt.Printf("%-20s  %7s  %7s  %9s  %9s  %5s\n",
		"Strategy", "Fitted", "Unfit", "Vol Used%", "Wt Used%", "Bins")
	fmt.Printf("%-20s  %7s  %7s  %9s  %9s  %5s\n",
		"--------------------", "-------", "-------", "---------", "---------", "-----")

	for _, si := range strategies {
		result := runStrategy(si.typ)
		fmt.Printf("%-20s  %7d  %7d  %8.1f%%  %8.1f%%  %5d\n",
			si.name,
			result.Stats.FittedItems,
			result.Stats.UnfittedCount,
			result.Stats.VolumeUsedPct,
			result.Stats.WeightUsedPct,
			result.Stats.TotalBins)
	}

	fmt.Println()
	fmt.Println("Notes:")
	fmt.Println("  - MinimizeBins / BestFitDecreasing sort items by volume descending (large first)")
	fmt.Println("  - Greedy / BestFit / NextFit / WorstFit sort by volume ascending (small first)")
	fmt.Println("  - BestFit picks the bin with least remaining space")
	fmt.Println("  - WorstFit picks the bin with most remaining space")
	fmt.Println("  - AlmostWorstFit is like WorstFit but skips bins that are >80% empty")
	fmt.Println("  - NextFit only tries the current bin, never goes back")
}

// runStrategy creates a fresh packer, bins, and items for a given strategy.
func runStrategy(st strategy.Type) *model.Result {
	p := packer.NewPacker(packer.WithStrategy(st))

	// Three bins of varying sizes.
	p.AddBin(model.NewBin("bin-S", 40, 40, 40, 100))
	p.AddBin(model.NewBin("bin-M", 60, 60, 60, 200))
	p.AddBin(model.NewBin("bin-L", 80, 80, 80, 400))

	// A mix of items.
	items := []struct {
		id string
		w, h, d, wt float64
	}{
		{"pkg-01", 30, 25, 20, 10},
		{"pkg-02", 35, 30, 25, 15},
		{"pkg-03", 20, 15, 10, 5},
		{"pkg-04", 25, 20, 15, 8},
		{"pkg-05", 40, 35, 30, 20},
		{"pkg-06", 15, 10, 10, 3},
		{"pkg-07", 50, 40, 35, 25},
		{"pkg-08", 20, 20, 20, 7},
		{"pkg-09", 30, 30, 25, 12},
		{"pkg-10", 35, 25, 20, 14},
		{"pkg-11", 10, 10, 10, 2},
		{"pkg-12", 45, 35, 30, 18},
	}
	for _, it := range items {
		p.AddItem(model.NewItem(it.id, it.w, it.h, it.d, it.wt))
	}

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatalf("strategy %d failed: %v", st, err)
	}
	return result
}
