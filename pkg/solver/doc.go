// Package solver provides advanced search algorithms for finding optimal
// or near-optimal 3D bin packing solutions.
//
// Solvers implement the [Solver] interface, which accepts a context, a set of
// bins, and a set of items, returning a [model.Result] with the best packing
// found within the time budget.
//
// # BranchBound
//
// The [BranchBound] solver exhaustively searches for the item ordering that
// maximizes bin utilization. It processes bins sequentially, optimizing each
// one independently.
//
// Two variants are available:
//
// Fast variant (default): iterates over all item permutations, packing each
// permutation greedily with the provided placement engine. This is practical
// for up to approximately 12 items when no deadline is set. For larger sets,
// use a context with a timeout.
//
//	bb := solver.NewBranchBound(func() placement.Engine {
//	    return placement.NewPivotEngine()
//	})
//	result, err := bb.Solve(ctx, bins, items)
//
// Full variant: performs a depth-first search with per-item rotation selection,
// exploring all combinations of item orderings and rotations. More thorough
// but significantly slower; practical for up to approximately 8 items without
// a deadline. Enable it with the [BBFull] option:
//
//	bb := solver.NewBranchBound(engineFactory, solver.BBFull())
//
// Both variants respect context deadlines and cancellation, returning the best
// solution found so far when the context expires. A greedy seed solution is
// computed first, so there is always a valid result.
//
// # Parallel
//
// The [Parallel] solver runs multiple packing configurations concurrently and
// returns the best result. Each configuration specifies a placement engine
// factory and a packing strategy. Configurations run with independent deep
// copies of bins and items, so there is no shared mutable state.
//
//	ps := solver.NewParallel(
//	    solver.WithConfig(
//	        func() placement.Engine { return placement.NewPivotEngine() },
//	        strategy.BestFitDecreasing,
//	    ),
//	    solver.WithConfig(
//	        func() placement.Engine { return placement.NewExtremePointEngine() },
//	        strategy.MinimizeBins,
//	    ),
//	)
//	result, err := ps.Solve(ctx, bins, items)
//
// If no configurations are provided, [NewParallel] uses a default set of 5
// built-in configurations covering all three placement engines and key
// strategies.
//
// The best result is selected by comparing the number of fitted items first,
// then average volume utilization as a tiebreaker.
package solver
