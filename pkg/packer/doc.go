// Package packer provides the public API for GoPackX, a 3D bin packing library.
//
// The [Packer] is the main entry point for packing items into bins. It
// orchestrates a placement engine (how items are positioned in 3D space) with
// a strategy (how items are ordered and bins are selected).
//
// # Quick Start
//
//	p := packer.NewPacker()
//	p.AddBin(model.NewBin("bin-1", 100, 100, 100, 500))
//	p.AddItem(model.NewItem("item-1", 50, 50, 50, 10))
//	p.AddItem(model.NewItem("item-2", 40, 40, 40, 8))
//
//	result, err := p.Pack(context.Background())
//	fmt.Println(result.Stats.FittedItems) // number of items packed
//
// # Configuration
//
// The packer is configured via functional options passed to [NewPacker]:
//
//   - [WithStrategy]: selects the item ordering and bin selection strategy
//     (default: [strategy.BestFitDecreasing]).
//   - [WithPlacementEngine]: selects the 3D placement algorithm
//     (default: [placement.NewPivotEngine]).
//
// Example with custom strategy and engine:
//
//	p := packer.NewPacker(
//	    packer.WithStrategy(strategy.MinimizeBins),
//	    packer.WithPlacementEngine(placement.NewExtremePointEngine()),
//	)
//
// # Strategies
//
// Strategies control item sort order (by volume ascending or descending) and
// bin selection logic (first fit, best fit, worst fit, etc.). See the
// [strategy] package for the full list.
//
// # Placement Engines
//
// Engines handle the geometric placement of items within a bin. The
// [placement] package provides three engines: PivotEngine, ExtremePointEngine,
// and LAFFEngine, each with different trade-offs between speed and packing quality.
//
// # Context Support
//
// [Packer.Pack] accepts a [context.Context] for cancellation and deadline support.
// If the context is cancelled, the packer stops placing items and returns the
// partial result.
package packer
