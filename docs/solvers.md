# Solvers

Solvers go beyond the standard `Packer` by searching over multiple configurations or item orderings to find better packing solutions. They implement the `Solver` interface:

```go
type Solver interface {
    Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error)
}
```

GoPackX provides two solvers: **Branch & Bound** for optimal single-bin packing, and **Parallel** for concurrent multi-configuration search.

## Branch & Bound

The Branch & Bound solver exhaustively searches for the item ordering that maximizes bin utilization. It processes bins **sequentially**, optimizing the packing of each bin before moving to the next.

```go
import (
    "github.com/jcoruiz/gopackx/pkg/solver"
    "github.com/jcoruiz/gopackx/pkg/placement"
)

bb := solver.NewBranchBound(func() placement.Engine {
    return placement.NewPivotEngine()
})

result, err := bb.Solve(ctx, bins, items)
```

### Engine Factory

Branch & Bound requires an **engine factory** (`func() placement.Engine`) instead of a shared engine instance. This is because the solver needs fresh engine instances for each search branch to avoid state contamination (especially important for stateful engines like ExtremePoints and LAFF).

```go
// Correct: factory function creates a new engine each time
bb := solver.NewBranchBound(func() placement.Engine {
    return placement.NewExtremePointEngine()
})

// With stability
bb := solver.NewBranchBound(func() placement.Engine {
    return placement.NewPivotEngine(placement.WithStability(0.7))
})
```

### Fast Variant (Default)

The default fast variant iterates over all **permutations** of the item list, packing each permutation greedily. It finds the ordering that fits the most items into each bin.

```go
bb := solver.NewBranchBound(func() placement.Engine {
    return placement.NewPivotEngine()
})
```

- **Algorithm**: Tries all item orderings using the next-permutation algorithm. For each permutation, items are packed greedily into the bin.
- **Seeds**: Starts with a greedy baseline (items in their current order). Improves from there.
- **Pruning**: Stops early when all items fit.
- **Complexity**: O(n!) permutations. Practical for **12 items or fewer** without a timeout.
- **Benchmark**: 6 items ~5.3us, 8 items ~10.6us

### Full Variant

The full variant uses **depth-first search** with per-item rotation selection. At each level of the search tree, it tries every unused item in every allowed rotation.

```go
bb := solver.NewBranchBound(
    func() placement.Engine { return placement.NewPivotEngine() },
    solver.BBFull(),
)
```

- **Algorithm**: DFS backtracking. At depth `d`, picks an unused item, fixes its rotation, places it, then recurses. Backtracks to try other items and rotations.
- **Seeds**: Same greedy baseline as the fast variant.
- **Pruning**: Count-based pruning -- skips branches that cannot beat the current best even if all remaining items fit. Stops when optimal solution (all items placed) is found.
- **Complexity**: O(n! * r^n) where r is the average number of allowed rotations. Practical for **8 items or fewer** without a timeout.
- **Benchmark**: 6 items ~5.2us

### Context and Timeouts

Both variants respect `context.Context`. When the deadline expires or the context is cancelled, the solver returns the **best solution found so far**.

```go
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()

bb := solver.NewBranchBound(func() placement.Engine {
    return placement.NewPivotEngine()
})

// Returns best result found within 100ms
result, err := bb.Solve(ctx, bins, items)
```

This makes Branch & Bound safe for production use even with larger item sets -- set a timeout and you get the best answer found within the time budget.

### How Bins Are Processed

Branch & Bound processes bins **sequentially**:

1. Optimize packing for the first bin (find best item ordering)
2. Items that were placed in bin 1 are removed from the pool
3. Optimize packing for the second bin with remaining items
4. Repeat until all bins are processed or all items are placed

## Parallel Solver

The Parallel solver runs multiple packing configurations **concurrently** and returns the best result. Each configuration is a combination of a placement engine and a strategy.

```go
// Use default configurations (5 combos)
ps := solver.NewParallel()
result, err := ps.Solve(ctx, bins, items)
```

### Default Configurations

When no custom configs are provided, the Parallel solver uses 5 default configurations:

| # | Engine | Strategy |
|---|---|---|
| 1 | Pivot | BestFitDecreasing |
| 2 | Pivot | MinimizeBins |
| 3 | ExtremePoints | BestFitDecreasing |
| 4 | LAFF | BestFitDecreasing |
| 5 | LAFF-Fast | BestFitDecreasing |

### Custom Configurations

Override the defaults with `WithConfig`:

```go
ps := solver.NewParallel(
    solver.WithConfig(
        func() placement.Engine { return placement.NewPivotEngine() },
        strategy.BestFitDecreasing,
    ),
    solver.WithConfig(
        func() placement.Engine { return placement.NewExtremePointEngine() },
        strategy.BestFitDecreasing,
    ),
    solver.WithConfig(
        func() placement.Engine { return placement.NewLAFFEngine(placement.LAFFFast()) },
        strategy.MinimizeBins,
    ),
)
```

### Thread Safety

Each configuration gets a **deep copy** of all bins and items. Goroutines work completely independently -- no shared mutable state. The engine factory (`func() placement.Engine`) creates a fresh engine instance per goroutine.

### Result Selection

The solver picks the best result using two criteria (in order):

1. **Most fitted items** (higher is better)
2. **Highest volume utilization** (tiebreaker)

### Performance

The Parallel solver's total time is bounded by the **slowest** configuration:

- With default configs: ~17ms (bottlenecked by ExtremePoints)
- Without ExtremePoints: ~1.5ms (bottlenecked by Pivot)
- LAFF-only configs: <0.5ms

Memory usage is the **sum** of all configurations since they run concurrently. With default configs, ~975KB total.

### Example

```go
import (
    "context"
    "time"

    "github.com/jcoruiz/gopackx/pkg/model"
    "github.com/jcoruiz/gopackx/pkg/placement"
    "github.com/jcoruiz/gopackx/pkg/solver"
    "github.com/jcoruiz/gopackx/pkg/strategy"
)

func main() {
    bins := []*model.Bin{
        model.NewBin("bin-1", 100, 100, 100, 500),
        model.NewBin("bin-2", 100, 100, 100, 500),
    }

    items := []*model.Item{
        model.NewItem("A", 50, 50, 50, 20),
        model.NewItem("B", 40, 30, 30, 15),
        model.NewItem("C", 30, 20, 20, 10),
        // ... more items
    }

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    ps := solver.NewParallel()
    result, err := ps.Solve(ctx, bins, items)
    if err != nil {
        // handle error
    }

    fmt.Printf("Fitted: %d/%d, Utilization: %.1f%%\n",
        result.Stats.FittedItems,
        result.Stats.TotalItems,
        result.Stats.VolumeUsedPct)
}
```

## Combining Solvers

You can use Branch & Bound inside Parallel solver configs by wrapping it in a custom solver, or by using Branch & Bound as a preprocessing step.

A practical pattern is to use the Parallel solver for the initial pass, then use Branch & Bound on bins that have unfitted items:

```go
// Step 1: quick parallel search
ps := solver.NewParallel()
result, _ := ps.Solve(ctx, bins, items)

// Step 2: if items didn't fit, try BB on a fresh bin
if len(result.UnfittedItems) > 0 && len(result.UnfittedItems) <= 12 {
    extraBin := model.NewBin("overflow", 100, 100, 100, 500)
    bb := solver.NewBranchBound(func() placement.Engine {
        return placement.NewPivotEngine()
    })

    bbCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
    defer cancel()

    bbResult, _ := bb.Solve(bbCtx, []*model.Bin{extraBin}, result.UnfittedItems)
    // Use bbResult for the overflow bin
}
```

## When to Use Which

| Scenario | Recommendation |
|---|---|
| **General purpose (>90% of cases)** | Standard `Packer` with defaults |
| **Don't know which engine/strategy is best** | Parallel solver (tries all, picks best) |
| **Quality matters, multiple cores available** | Parallel solver with custom configs |
| **Small item set (8-12 items), optimal packing critical** | Branch & Bound (fast) with timeout |
| **Very small item set (up to 8 items), exhaustive search** | Branch & Bound (full) |
| **High throughput, many packing operations** | Standard `Packer` with LAFF-Fast engine |
| **Post-processing overflow items** | Branch & Bound on unfitted items |

## Benchmark Summary

All benchmarks run on AMD Ryzen 9 9950X3D:

| Solver | Items | Time | Memory | Allocs |
|---|---|---|---|---|
| BB Fast | 6 | ~5.3us | 6.6KB | 77 |
| BB Fast | 8 | ~10.6us | 9.9KB | 111 |
| BB Full | 6 | ~5.2us | 6.6KB | 77 |
| Parallel (default, 5 configs) | 50 | ~17ms | 975KB | 6551 |
