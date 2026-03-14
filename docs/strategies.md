# Packing Strategies

GoPackX provides 7 built-in strategies that control two things:

1. **Item sort order** -- which items get packed first
2. **Bin selection** -- which bin an item is tried in first

The strategy is set when creating a packer:

```go
import (
    "github.com/jcoruiz/gopackx/pkg/packer"
    "github.com/jcoruiz/gopackx/pkg/strategy"
)

p := packer.NewPacker(
    packer.WithStrategy(strategy.BestFitDecreasing),
)
```

## How Item Sorting Works

All strategies use a multi-criteria stable sort with four tiebreakers:

1. **Volume** -- direction depends on strategy (ascending or descending)
2. **Load-bearing capacity** -- descending (sturdy items first)
3. **Priority** -- ascending (1 = highest priority, packed first)
4. **Group** -- alphabetical (keeps binding groups together)

Strategies that sort by **volume descending** (largest first): `BestFitDecreasing`, `MinimizeBins`.

Strategies that sort by **volume ascending** (smallest first): `BestFit`, `Greedy`, `NextFit`, `WorstFit`, `AlmostWorstFit`.

## Strategy Reference

### BestFitDecreasing (Default)

```go
packer.WithStrategy(strategy.BestFitDecreasing)
```

- **Item order**: Volume descending (largest items first)
- **Bin selection**: Smallest remaining volume that can accept the item
- **How it works**: Large items are placed first when there is the most space. Each item goes into the tightest-fitting bin, reducing wasted space.
- **When to use**: General-purpose default. Works well for mixed item sizes. Best overall volume utilization in most scenarios.
- **Trade-offs**: Slightly slower than First Fit strategies due to bin sorting, but produces significantly better packing.

### MinimizeBins

```go
packer.WithStrategy(strategy.MinimizeBins)
```

- **Item order**: Volume descending (largest items first)
- **Bin selection**: First Fit (bins tried in original order)
- **How it works**: Like BestFitDecreasing, large items go first. But instead of finding the tightest bin, items go into the first bin that accepts them. This fills bins sequentially.
- **When to use**: When minimizing the number of containers used matters more than perfect utilization within each container.
- **Trade-offs**: May leave more gaps within individual bins, but tends to use fewer total bins.

### BestFit

```go
packer.WithStrategy(strategy.BestFit)
```

- **Item order**: Volume ascending (smallest items first)
- **Bin selection**: Smallest remaining volume that can accept the item
- **How it works**: Small items fill gaps first, then larger items are placed. Each item targets the bin with the least remaining space.
- **When to use**: When you want to leave space in bins for larger items that will be added later, or when processing items from a stream where large items arrive last.
- **Trade-offs**: Packing small items first can create fragmented spaces that are hard to use for larger items.

### Greedy

```go
packer.WithStrategy(strategy.Greedy)
```

- **Item order**: Volume ascending (smallest items first)
- **Bin selection**: First Fit (bins tried in original order)
- **How it works**: Takes items smallest-first and places each into the first available bin. No optimization of bin choice.
- **When to use**: When speed matters more than packing quality, or as a baseline to compare against other strategies.
- **Trade-offs**: Fastest bin-selection logic but worst utilization. Small items may fill spaces that larger items needed.

### NextFit

```go
packer.WithStrategy(strategy.NextFit)
```

- **Item order**: Volume ascending (smallest items first)
- **Bin selection**: Current bin only (never revisits previous bins)
- **How it works**: Maintains a pointer to the current bin. If an item fits, it goes in. If not, the pointer advances to the next bin and never goes back.
- **When to use**: Streaming scenarios where you need O(n) time complexity and can tolerate using more bins.
- **Trade-offs**: Fastest strategy overall (single pass, no bin sorting). Uses the most bins because it never revisits partially filled containers.

### WorstFit

```go
packer.WithStrategy(strategy.WorstFit)
```

- **Item order**: Volume ascending (smallest items first)
- **Bin selection**: Largest remaining volume (most empty bin first)
- **How it works**: Spreads items across bins by always targeting the bin with the most free space. This distributes load evenly.
- **When to use**: When you need balanced weight/volume distribution across containers (e.g., even loads on delivery trucks).
- **Trade-offs**: Uses more bins than BestFit strategies. Individual bins have lower utilization.

### AlmostWorstFit

```go
packer.WithStrategy(strategy.AlmostWorstFit)
```

- **Item order**: Volume ascending (smallest items first)
- **Bin selection**: Worst Fit, but skips bins that are less than 20% full (more than 80% empty)
- **How it works**: Like WorstFit, but avoids spreading items into nearly empty bins. This is a compromise -- it spreads load across partially used bins while letting mostly empty bins stay available for larger items.
- **When to use**: When you want load balancing but also want to avoid opening new bins unnecessarily.
- **Trade-offs**: Better bin utilization than pure WorstFit. Falls back to regular WorstFit if all candidates are very empty.

## Choosing a Strategy

Use this decision guide to pick the right strategy:

```
Do you need the fastest possible packing?
  YES -> Is O(n) time critical?
           YES -> NextFit
           NO  -> Greedy
  NO  -> Do you need balanced load across bins?
           YES -> Is bin count a concern?
                    YES -> AlmostWorstFit
                    NO  -> WorstFit
           NO  -> Do you process items smallest-first?
                    YES -> BestFit
                    NO  -> Is minimizing bin count the priority?
                             YES -> MinimizeBins
                             NO  -> BestFitDecreasing (default)
```

### Summary Table

| Strategy | Item Sort | Bin Selection | Best For |
|---|---|---|---|
| **BestFitDecreasing** | Vol desc | Smallest remaining | General purpose (default) |
| **MinimizeBins** | Vol desc | First Fit | Minimizing container count |
| **BestFit** | Vol asc | Smallest remaining | Leaving room for late large items |
| **Greedy** | Vol asc | First Fit | Speed over quality |
| **NextFit** | Vol asc | Current bin only | Streaming, O(n) |
| **WorstFit** | Vol asc | Largest remaining | Even load distribution |
| **AlmostWorstFit** | Vol asc | Largest remaining (skip >80% empty) | Balanced spreading |

## Strategy with Engines and Solvers

Strategies work with any placement engine:

```go
p := packer.NewPacker(
    packer.WithStrategy(strategy.MinimizeBins),
    packer.WithPlacementEngine(placement.NewExtremePointEngine()),
)
```

The Parallel solver can try multiple strategy+engine combinations concurrently:

```go
s := solver.NewParallel(
    solver.WithConfig(
        func() placement.Engine { return placement.NewPivotEngine() },
        strategy.BestFitDecreasing,
    ),
    solver.WithConfig(
        func() placement.Engine { return placement.NewLAFFEngine() },
        strategy.MinimizeBins,
    ),
)
```

See [Solvers](solvers.md) for details.

## Benchmark Context

Strategy choice has minimal impact on raw packing time -- the placement engine dominates. All benchmarks below used 50 items with 3 bins of 100x100x100 (BestFitDecreasing strategy):

| Engine | Time per pack |
|---|---|
| LAFF-Fast | ~0.25ms |
| LAFF | ~0.38ms |
| Pivot | ~1.5ms |
| ExtremePoints | ~17ms |

The strategy primarily affects **packing quality** (utilization, bin count), not speed. Choose based on your quality requirements, then pick an engine for the speed/quality trade-off you need.
