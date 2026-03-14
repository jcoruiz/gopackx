# Performance Guide

This guide covers benchmark data, engine selection, scaling behavior, and practical tips for optimizing GoPackX performance.

## Benchmark Results

All benchmarks were run on an **AMD Ryzen 9 9950X3D** (16-core), using **50 items** packed into **3 bins of 100x100x100** with the BestFitDecreasing strategy.

### Placement Engines

| Engine | Time/op | Allocs/op | Memory/op |
|---|---|---|---|
| LAFF-Fast | 0.25ms | 377 | 115KB |
| LAFF | 0.38ms | 400 | 170KB |
| Pivot | 1.5ms | 1,991 | 192KB |
| ExtremePoints | 17ms | 1,827 | 304KB |

Key observations:

- LAFF-Fast is **6x faster** than Pivot and **68x faster** than ExtremePoints
- LAFF variants use far fewer allocations (377-400 vs ~1900)
- ExtremePoints uses the most memory but fewer allocations than Pivot (larger individual allocations)
- Pivot offers a middle ground in all metrics

### Solvers

| Solver | Items | Time/op | Allocs/op | Memory/op |
|---|---|---|---|---|
| BB Fast | 6 | 5.3us | 77 | 6.6KB |
| BB Fast | 8 | 10.6us | 111 | 9.9KB |
| BB Full | 6 | 5.2us | 77 | 6.6KB |
| Parallel (default 5 configs) | 50 | 17ms | 6,551 | 975KB |

Key observations:

- Branch & Bound is extremely fast for small item sets (microseconds)
- BB Fast and BB Full have similar performance for 6 items -- the greedy seed already finds a good solution
- BB Fast scales to 8 items at ~2x the cost of 6 items (but grows factorially beyond that)
- Parallel solver time equals the slowest config (~17ms = ExtremePoints bottleneck)
- Parallel memory is roughly the sum of all configs running concurrently

## Choosing the Right Engine

```
What is your primary concern?
  |
  +-- Throughput (process many packing operations)
  |     -> LAFF-Fast (~0.25ms per pack)
  |
  +-- Balanced speed and quality
  |     -> Pivot (~1.5ms per pack) -- the default
  |
  +-- Best possible packing quality
  |     -> ExtremePoints (~17ms per pack)
  |
  +-- Not sure / depends on data
        -> Parallel solver (tries all, picks best)
```

### Speed vs Quality Trade-off

```
Quality  ^
         |  * ExtremePoints
         |
         |        * Pivot
         |
         |  * LAFF
         |  * LAFF-Fast
         +-------------------------> Speed
            Fast                Slow
```

ExtremePoints produces the densest packing because it generates more candidate positions (intersection points between items) and uses multi-criteria scoring. Pivot uses simpler corner-based candidates. LAFF sacrifices inter-level optimization for speed.

## Scaling Considerations

### Pivot Engine -- O(n^2)

Each new item generates 3 pivot points per placed item. Each candidate position checks for intersection against all placed items. Total work: `n * (3n) * n = O(n^3)` worst case, typically closer to `O(n^2)` because most candidates are rejected early.

- **Sweet spot**: up to a few hundred items
- **Degrades at**: 500+ items (intersection checks dominate)

### Extreme Points Engine -- O(n^2)

After each placement, all extreme points have their `MaxSpace` recalculated against all placed items. Point generation also checks intersections with all items.

- **Sweet spot**: under 100 items
- **Degrades at**: 100+ items (point maintenance becomes expensive)

### LAFF Engine -- O(n * levels)

Each item tries placement in existing levels. Within a level, only items at the same Y position are checked. New levels add constant overhead.

- **Sweet spot**: up to thousands of items
- **Degrades at**: very large item count with many levels (rare in practice)

### Branch & Bound -- O(n!)

The fast variant tries all permutations (n!). The full variant tries all permutations times all rotation combinations (n! * r^n). Growth is extremely fast.

| Items | Permutations | Fast BB Time (approx) |
|---|---|---|
| 6 | 720 | ~5us |
| 8 | 40,320 | ~11us |
| 10 | 3,628,800 | ~1ms |
| 12 | 479,001,600 | ~100ms |
| 15 | 1.3 trillion | use timeout |

**Always use a context timeout** for item counts above 12:

```go
ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
defer cancel()
result, _ := bb.Solve(ctx, bins, items)
```

The solver returns the best solution found before the deadline.

## Context and Timeouts

All packing operations (`Packer.Pack`, `Solver.Solve`) accept a `context.Context` and check for cancellation at each item placement.

```go
// Standard packer with timeout
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()
result, err := p.Pack(ctx)
if err != nil {
    // err == context.DeadlineExceeded if timeout hit
}

// Branch & Bound with timeout
ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()
result, err = bb.Solve(ctx, bins, items)
// result contains best solution found within the time budget, even if err != nil
```

For the standard `Packer`, context cancellation stops the loop mid-way -- already-placed items stay placed, remaining items go to `UnfittedItems`.

For Branch & Bound, cancellation stops the search and returns the best-so-far solution. This is the recommended way to use BB with larger item sets.

## Memory Optimization

### Zero Dependencies

GoPackX has no external dependencies -- only Go's standard library. No hidden allocations from third-party code.

### Pointer-Based Items and Bins

Items and bins are pointer types (`*Item`, `*Bin`). The packer works with pointer slices and does not copy item data during normal packing. Modification happens in-place (setting `Position`, `RotationType`, `Placed`).

### Solver Deep Copies

Solvers (`Parallel`, `BranchBound`) internally deep-copy bins and items to ensure thread safety and support backtracking. This is the main source of allocations in solver benchmarks:

- Parallel: 5 configs * (bins + items) copies = ~975KB for 50 items
- BB Fast: copies items for each permutation attempt
- BB Full: copies bins at each DFS level for backtracking

### Engine State

- **Pivot**: Stateless -- no per-bin state. Pivot points are generated fresh for each `PlaceItem` call.
- **ExtremePoints**: Stateful -- maintains a list of candidate points per bin. Reinitializes when the bin pointer changes. Memory grows with item count.
- **LAFF**: Stateful -- tracks levels per bin. Reinitializes when the bin pointer changes. Lightweight state.

## Practical Tips

### Start with Defaults

The default configuration (Pivot + BestFitDecreasing) handles most workloads well:

```go
p := packer.NewPacker()
```

Only change engines/strategies when profiling shows the default isn't meeting your needs.

### Use Parallel Solver When Quality Matters

If you need the best possible packing and have CPU cores to spare:

```go
ps := solver.NewParallel()
result, _ := ps.Solve(ctx, bins, items)
```

The ~17ms cost is negligible for most applications, and you get the benefit of 5 different approaches.

### Use LAFF-Fast for High Throughput

For batch processing, real-time systems, or when you need to pack thousands of operations per second:

```go
p := packer.NewPacker(
    packer.WithPlacementEngine(placement.NewLAFFEngine(placement.LAFFFast())),
)
```

At 0.25ms per pack, you can process ~4,000 packing operations per second on a single core.

### Use Branch & Bound for Small Critical Sets

When you have 12 or fewer items and optimal packing is critical:

```go
ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
defer cancel()

bb := solver.NewBranchBound(func() placement.Engine {
    return placement.NewPivotEngine()
})
result, _ := bb.Solve(ctx, bins, items)
```

### Enable Stability Only When Needed

Stability checking adds overhead because each placement must calculate support ratios and check load-bearing constraints:

```go
// Only enable if physical stability matters
engine := placement.NewPivotEngine(placement.WithStability(0.7))
```

If you are packing for volume optimization only (e.g., storage planning) and don't care about physical stacking, leave stability disabled.

### Trim Parallel Configs for Speed

If you know ExtremePoints is too slow for your use case, create custom Parallel configs without it:

```go
ps := solver.NewParallel(
    solver.WithConfig(
        func() placement.Engine { return placement.NewPivotEngine() },
        strategy.BestFitDecreasing,
    ),
    solver.WithConfig(
        func() placement.Engine { return placement.NewPivotEngine() },
        strategy.MinimizeBins,
    ),
    solver.WithConfig(
        func() placement.Engine { return placement.NewLAFFEngine() },
        strategy.BestFitDecreasing,
    ),
    solver.WithConfig(
        func() placement.Engine { return placement.NewLAFFEngine(placement.LAFFFast()) },
        strategy.BestFitDecreasing,
    ),
)
// Now bottlenecked by Pivot (~1.5ms) instead of ExtremePoints (~17ms)
```

### Profile Before Optimizing

Use Go's built-in profiling to understand where time is spent:

```bash
go test -bench=BenchmarkPack50Items -cpuprofile=cpu.prof -memprofile=mem.prof ./pkg/packer/
go tool pprof cpu.prof
```

Common hotspots:

- `intersection.Intersect` -- AABB collision checks (dominates Pivot engine)
- `ExtremePointEngine.recalculateMaxSpace` -- point maintenance (dominates EP engine)
- `stability.SupportRatio` -- support calculation (when stability enabled)

## Quick Reference

| Goal | Configuration |
|---|---|
| Best general-purpose defaults | `packer.NewPacker()` |
| Maximum throughput | Pivot or LAFF-Fast engine |
| Best quality, time not critical | ExtremePoints engine or Parallel solver |
| Optimal packing, small item set | Branch & Bound with timeout |
| Balanced quality, multi-core | Parallel solver (default configs) |
| Balanced quality, single-core | Pivot + BestFitDecreasing |
| Physical stability required | Engine with `WithStability(0.7)` |
| Streaming / real-time | LAFF-Fast + NextFit strategy |
