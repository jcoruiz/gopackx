# Performance Guide

This guide covers benchmark data, engine selection, scaling behavior, and practical tips for optimizing GoPackX performance.

## Benchmark Results

All benchmarks were run on an **AMD Ryzen 9 9950X3D** (16-core) with Go 1.22, using the benchmarks in `pkg/packer` (50 items, BestFitDecreasing) and `pkg/solver`. v0.3.0 numbers; the v0.2 column is the same benchmark on v0.2.2.

### Placement Engines

| Engine | Time/op | v0.2 | Allocs/op | Memory/op |
|---|---|---|---|---|
| LAFF | 0.08ms | 0.09ms | 423 | 186KB |
| LAFF-Fast | 0.08ms | 0.06ms | 390 | 128KB |
| Pivot | 0.10ms | 0.08ms | 336 | 33KB |
| ExtremePoints | 1.4ms | 4.3ms | 1,901 | 386KB |
| MaxRects | 4.0ms | 0.08ms* | 675 | 286KB |

\* v0.2 MaxRects was fast because it lost most of its free spaces and placed far fewer items (79 of 400 in a pallet that holds them all).

Key observations:

- LAFF, LAFF-Fast and Pivot pack 50 items in about a tenth of a millisecond
- ExtremePoints and MaxRects are 14-40x slower and pack denser
- Pivot and LAFF-Fast are slightly slower than in v0.2: placement now also checks that a fragile or load-limited item is not slid under items placed earlier, and the Packer packs copies of your items
- ExtremePoints got faster, and scales much better: with TrialPacking, 800 items take about 2 seconds instead of 9 minutes (Apple M4 Pro)

### Solvers

| Solver | Items | Time/op | Allocs/op | Memory/op |
|---|---|---|---|---|
| BB Fast | 6 | 2.6us | 51 | 5.8KB |
| BB Fast | 8 | 4.0us | 62 | 8.2KB |
| BB Full | 6 | 2.6us | 51 | 5.8KB |
| Parallel (default 5 configs) | 50 | 1.8ms | 3,042 | 739KB |

Key observations:

- Branch & Bound is extremely fast for small item sets (microseconds)
- BB Fast and BB Full have similar performance for 6 items -- the greedy seed already finds a good solution
- BB Fast grows factorially beyond 8 items: use a deadline
- Parallel solver time equals the slowest config (ExtremePoints)
- Parallel memory is roughly the sum of all configs running concurrently

## Choosing the Right Engine

```
What is your primary concern?
  |
  +-- Throughput (process many packing operations)
  |     -> LAFF or LAFF-Fast (~0.08ms per pack)
  |
  +-- Balanced speed and quality
  |     -> Pivot (~0.1ms per pack) -- the default
  |
  +-- Best possible packing quality
  |     -> ExtremePoints (~1.4ms per pack) or MaxRects (~4ms)
  |
  +-- Not sure / depends on data
        -> Parallel solver (tries all, picks best)
```

### Speed vs Quality Trade-off

```
Quality  ^
         |  * ExtremePoints
         |  * MaxRects
         |
         |        * Pivot
         |
         |  * LAFF
         |  * LAFF-Fast
         +-------------------------> Speed
            Fast                Slow
```

ExtremePoints produces the densest packing because it generates more candidate positions (intersection points between items) and uses multi-criteria scoring. MaxRects comes close by tracking the maximal free spaces. Pivot uses simpler corner-based candidates. LAFF sacrifices inter-level optimization for speed.

## Scaling Considerations

### Pivot Engine -- O(n^2)

Each new item generates 3 pivot points per placed item. Each candidate position checks for intersection against all placed items. Total work: `n * (3n) * n = O(n^3)` worst case, typically closer to `O(n^2)` because most candidates are rejected early.

- **Sweet spot**: up to a few hundred items
- **Degrades at**: 500+ items (intersection checks dominate)

### Extreme Points Engine -- O(n^2)

After each placement, all extreme points have their `MaxSpace` recalculated against all placed items. Point generation also checks intersections with all items.

- **Sweet spot**: up to a few hundred items per bin
- **Degrades at**: about a thousand items in one bin (the point list grows with every item)

### MaxRects Engine -- O(n * spaces)

Each placement tries every free space with every rotation, and splits the spaces the new item overlaps.

- **Sweet spot**: up to a few hundred items per bin
- **Degrades at**: many small items in one large bin (the number of free spaces grows)

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

All packing operations (`gopackx.Pack`, `Packer.Pack`, `Solver.Solve`) accept a `context.Context` and check it between item placements, so they stop within one placement of the deadline.

When the context ends before they finish, they return the **best result found so far together with the context error**. Every item is in the result, placed or in `UnfittedItems`. When they finish in time, the error is nil.

```go
ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
defer cancel()
result, err := bb.Solve(ctx, bins, items)
if errors.Is(err, context.DeadlineExceeded) {
    // result holds the best packing found in 100ms
}
```

Deadlines also work in WebAssembly (`GOOS=js`). There, the timer behind `context.WithTimeout` cannot fire while a solver keeps the only thread busy, so GoPackX compares the deadline with the clock as well. CI runs the test suite as WebAssembly to keep it that way.

## Memory Optimization

### Zero Dependencies

GoPackX has no external dependencies -- only Go's standard library. No hidden allocations from third-party code.

### Pointer-Based Items and Bins

Items and bins are pointer types (`*Item`, `*Bin`). All entry points pack copies, so the bins and items you pass stay unchanged and the results are in `Result.Bins` and `Result.UnfittedItems`. Copying 50 items costs about 1% of a Pivot pack.

### Solver Deep Copies

Solvers (`Parallel`, `BranchBound`) internally deep-copy bins and items to ensure thread safety and support backtracking. This is the main source of allocations in solver benchmarks:

- Parallel: 5 configs * (bins + items) copies = ~975KB for 50 items
- BB Fast: copies items for each permutation attempt
- BB Full: copies bins at each DFS level for backtracking

### Engine State

- **Pivot**: Stateless -- no per-bin state. Pivot points are generated fresh for each `PlaceItem` call.
- **ExtremePoints**, **MaxRects**, **LAFF**: keep the state (points, free spaces, levels) of every bin they pack, so solvers can switch between open bins without rebuilding it. Memory grows with item count. Call `Reset()` before reusing an engine for an unrelated run; the Packer does it on every `Pack`.

### Load Tracking

Bins track the load on each item only once an item with a load limit is involved, so packing without load limits pays nothing for it.

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

The ~2ms cost for 50 items is negligible for most applications, and you get the benefit of 5 different approaches.

### Use LAFF-Fast for High Throughput

For batch processing, real-time systems, or when you need to pack thousands of operations per second:

```go
p := packer.NewPacker(
    packer.WithPlacementEngine(placement.NewLAFFEngine(placement.LAFFFast())),
)
```

At about 0.08ms per pack, you can process over 10,000 packing operations per second on a single core.

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

Stability checking adds overhead because each placement must calculate support ratios. (Load limits are checked whenever items have them, with or without stability.)

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
// Now bottlenecked by Pivot (~0.1ms) instead of ExtremePoints (~1.4ms)
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
