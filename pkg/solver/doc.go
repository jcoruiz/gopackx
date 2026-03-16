// Package solver provides advanced search algorithms for finding optimal
// or near-optimal 3D bin packing solutions.
//
// Solvers implement the [Solver] interface, which accepts a context, a set of
// bins, and a set of items, returning a [model.Result] with the best packing
// found within the time budget.
//
// For most use cases, the top-level [github.com/jcoruiz/gopackx.Pack] function
// is the simplest entry point. Use this package directly when you need
// fine-grained control over solver parameters.
//
// # TrialPacking (Variable-Sized Bin Packing)
//
// The [TrialPacking] solver addresses the Variable-Sized Bin Packing Problem
// (VSBPP): given multiple box types, select which types to use and how many of
// each, minimizing total boxes. It treats bins as templates and creates
// instances as needed.
//
// When a new bin is needed, it runs the actual placement engine on a temporary
// copy of each candidate bin type, measuring how many items really fit (not
// just volume estimates). It then picks the type with the best fill ratio.
//
//	tp := solver.NewTrialPacking(func() placement.Engine {
//	    return placement.NewPivotEngine()
//	})
//	result, err := tp.Solve(ctx, binTypes, items)
//
// Enable lookahead (Level 4) to also estimate how many future bins will be
// needed, leading to better global decisions:
//
//	tp := solver.NewTrialPacking(engineFactory, solver.WithLookahead())
//
// # Metaheuristic (Cross-Bin Optimization)
//
// The [Metaheuristic] solver uses Variable Neighborhood Search (VNS) to
// redistribute items across bins after an initial packing. It can find
// solutions that greedy approaches miss by trying MOVE, SWAP, REPACK, and
// CHANGE_TYPE operations, each validated with the real 3D placement engine.
//
//	m := solver.NewMetaheuristic(func() placement.Engine {
//	    return placement.NewPivotEngine()
//	})
//	result, err := m.Solve(ctx, binTypes, items)
//
// The metaheuristic starts from a TrialPacking seed and iteratively improves
// it. It is slower (~30ms for 17 items) but can reduce bin count beyond what
// greedy approaches achieve.
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
