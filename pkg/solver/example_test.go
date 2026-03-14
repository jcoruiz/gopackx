package solver_test

import (
	"context"
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/solver"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

func ExampleBranchBound_Solve() {
	bb := solver.NewBranchBound(func() placement.Engine {
		return placement.NewPivotEngine()
	})

	bins := []*model.Bin{
		model.NewBin("bin-1", 60, 60, 60, 500),
	}
	items := []*model.Item{
		model.NewItem("a", 30, 30, 30, 5),
		model.NewItem("b", 30, 30, 30, 5),
		model.NewItem("c", 20, 20, 20, 3),
	}

	result, err := bb.Solve(context.Background(), bins, items)
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

func ExampleParallel_Solve() {
	ps := solver.NewParallel(
		solver.WithConfig(
			func() placement.Engine { return placement.NewPivotEngine() },
			strategy.BestFitDecreasing,
		),
		solver.WithConfig(
			func() placement.Engine { return placement.NewExtremePointEngine() },
			strategy.MinimizeBins,
		),
	)

	bins := []*model.Bin{
		model.NewBin("bin-1", 100, 100, 100, 1000),
	}
	items := []*model.Item{
		model.NewItem("a", 50, 50, 50, 10),
		model.NewItem("b", 50, 50, 50, 10),
		model.NewItem("c", 40, 40, 40, 8),
	}

	result, err := ps.Solve(context.Background(), bins, items)
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
