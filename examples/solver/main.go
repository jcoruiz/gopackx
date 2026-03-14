// Example: advanced solvers
//
// Demonstrates the Branch & Bound solver (fast and full variants),
// the Parallel solver with default configs, and the Parallel solver
// with custom configs. Uses context.WithTimeout to show deadline support.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/solver"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

func main() {
	fmt.Println("=== Advanced Solvers ===")
	fmt.Println()

	// --- 1. Branch & Bound (fast variant) ---
	demoBBFast()

	// --- 2. Branch & Bound (full variant) ---
	demoBBFull()

	// --- 3. Parallel solver with default configs ---
	demoParallelDefault()

	// --- 4. Parallel solver with custom configs ---
	demoParallelCustom()
}

// makeItems creates a fresh set of items for each solver run.
func makeItems() []*model.Item {
	return []*model.Item{
		model.NewItem("A", 30, 25, 20, 10),
		model.NewItem("B", 35, 30, 25, 15),
		model.NewItem("C", 20, 15, 10, 5),
		model.NewItem("D", 25, 20, 15, 8),
		model.NewItem("E", 40, 35, 30, 20),
		model.NewItem("F", 15, 10, 10, 3),
		model.NewItem("G", 20, 20, 20, 7),
		model.NewItem("H", 10, 10, 10, 2),
	}
}

// makeBins creates a fresh set of bins for each solver run.
func makeBins() []*model.Bin {
	return []*model.Bin{
		model.NewBin("bin-1", 60, 60, 60, 200),
		model.NewBin("bin-2", 60, 60, 60, 200),
	}
}

func printResult(name string, result *model.Result, elapsed time.Duration) {
	fmt.Printf("  Fitted: %d / %d items\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("  Bins used: %d\n", result.Stats.TotalBins)
	fmt.Printf("  Avg volume used: %.1f%%\n", result.Stats.VolumeUsedPct)
	fmt.Printf("  Time: %s\n", elapsed.Round(time.Microsecond))
	if len(result.UnfittedItems) > 0 {
		fmt.Printf("  Unfitted: ")
		for i, item := range result.UnfittedItems {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Print(item.ID)
		}
		fmt.Println()
	}
	fmt.Println()
}

func demoBBFast() {
	fmt.Println("--- 1. Branch & Bound (Fast) ---")
	fmt.Println("Tries all item permutations, packing greedily per permutation.")
	fmt.Println()

	// Engine factory: creates a fresh PivotEngine for each search branch.
	newEngine := func() placement.Engine {
		return placement.NewPivotEngine()
	}

	bb := solver.NewBranchBound(newEngine)

	// Use a 2-second timeout to demonstrate deadline support.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	result, err := bb.Solve(ctx, makeBins(), makeItems())
	elapsed := time.Since(start)
	if err != nil {
		log.Printf("  B&B fast: %v (partial result used)\n", err)
	}

	printResult("B&B Fast", result, elapsed)
}

func demoBBFull() {
	fmt.Println("--- 2. Branch & Bound (Full) ---")
	fmt.Println("DFS with per-item rotation selection. More thorough but slower.")
	fmt.Println()

	newEngine := func() placement.Engine {
		return placement.NewPivotEngine()
	}

	// BBFull() enables the full variant.
	bb := solver.NewBranchBound(newEngine, solver.BBFull())

	// Shorter timeout since full search is more expensive.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	result, err := bb.Solve(ctx, makeBins(), makeItems())
	elapsed := time.Since(start)
	if err != nil {
		log.Printf("  B&B full: %v (partial result used)\n", err)
	}

	printResult("B&B Full", result, elapsed)
}

func demoParallelDefault() {
	fmt.Println("--- 3. Parallel Solver (Default Configs) ---")
	fmt.Println("Runs multiple engine+strategy combos concurrently, picks the best.")
	fmt.Println()

	// NewParallel with no options uses the default config set:
	// Pivot+BFD, Pivot+MinBins, EP+BFD, LAFF+BFD, LAFFFast+BFD
	ps := solver.NewParallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, err := ps.Solve(ctx, makeBins(), makeItems())
	elapsed := time.Since(start)
	if err != nil {
		log.Printf("  Parallel default: %v\n", err)
	}

	printResult("Parallel (default)", result, elapsed)
}

func demoParallelCustom() {
	fmt.Println("--- 4. Parallel Solver (Custom Configs) ---")
	fmt.Println("Runs user-defined engine+strategy combinations.")
	fmt.Println()

	// Custom configurations: try ExtremePoint with three different strategies.
	ps := solver.NewParallel(
		solver.WithConfig(
			func() placement.Engine { return placement.NewExtremePointEngine() },
			strategy.BestFitDecreasing,
		),
		solver.WithConfig(
			func() placement.Engine { return placement.NewExtremePointEngine() },
			strategy.MinimizeBins,
		),
		solver.WithConfig(
			func() placement.Engine { return placement.NewLAFFEngine() },
			strategy.Greedy,
		),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	result, err := ps.Solve(ctx, makeBins(), makeItems())
	elapsed := time.Since(start)
	if err != nil {
		log.Printf("  Parallel custom: %v\n", err)
	}

	printResult("Parallel (custom)", result, elapsed)
}
