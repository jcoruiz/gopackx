# Placement Engines

A placement engine determines **where** inside a bin each item is physically positioned. GoPackX provides four engines (Pivot Points, Extreme Points, MaxRects and LAFF, which has a fast variant), each implementing the `placement.Engine` interface:

```go
type Engine interface {
    PlaceItem(bin *model.Bin, item *model.Item) bool
}
```

The engine is set when creating a packer:

```go
import (
    "github.com/jcoruiz/gopackx/pkg/packer"
    "github.com/jcoruiz/gopackx/pkg/placement"
)

// Default: Pivot Points
p := packer.NewPacker()

// Explicit engine selection
p = packer.NewPacker(
    packer.WithPlacementEngine(placement.NewExtremePointEngine()),
)
```

## Pivot Points Engine (Default)

```go
placement.NewPivotEngine()
placement.NewPivotEngine(placement.WithStability(0.7))
```

### Algorithm

The Pivot Points engine generates candidate placement positions from the corners of already-placed items.

1. **First item**: placed at the origin `(0, 0, 0)`
2. **Subsequent items**: for each already-placed item, three pivot points are generated from its far corners:
   - `(x + width, y, z)` -- to the right
   - `(x, y + height, z)` -- on top
   - `(x, y, z + depth)` -- behind
3. The origin `(0, 0, 0)` is always included as a candidate
4. For each pivot point, all allowed rotations are tried
5. The first valid placement is accepted

After a valid placement is found, **fix-point correction** pushes the item toward the origin along each axis (Y first for gravity, then X, then Z). This compaction step eliminates floating gaps and produces tighter packing. If the corrected position is invalid (e.g., fails stability checks), the original position is kept.

### Configuration

| Option | Description |
|---|---|
| `WithStability(ratio)` | Enable stability checking. Items must have at least `ratio` fraction of their base area supported by surfaces below. |

### Characteristics

- **Speed**: ~0.1ms for 50 items (336 allocs, 33KB)
- **Quality**: Good -- fix-point correction produces reasonably tight packing
- **Scaling**: O(n^2) placement checks (each new item checks against all placed items)
- **State**: Stateless -- generates pivots fresh for each placement call
- **Best for**: General-purpose use, good balance of speed and quality

### Example

```go
engine := placement.NewPivotEngine(placement.WithStability(0.7))

p := packer.NewPacker(
    packer.WithPlacementEngine(engine),
)

bin := model.NewBin("bin-1", 100, 100, 100, 500)
p.AddBin(bin)

p.AddItem(model.NewItem("A", 50, 50, 50, 20))
p.AddItem(model.NewItem("B", 40, 30, 30, 15))

result, _ := p.Pack(context.Background())
```

## Extreme Points Engine

```go
placement.NewExtremePointEngine()
placement.NewExtremePointEngine(placement.WithEPStability(0.7))
```

### Algorithm

The Extreme Points engine maintains a dynamic list of candidate positions, each annotated with metadata about available space and surface support.

1. **Initialization**: a single extreme point at `(0, 0, 0)` with max space equal to the full bin dimensions and 3 supporting planes (floor + two walls)
2. **Placement scoring**: for each item rotation, all extreme points are evaluated:
   - **Quick rejection**: if the item's dimensions exceed the point's `MaxSpace` in any axis, skip immediately (no need for full collision checks)
   - **Score calculation** (lower is better):
     - Support penalty: `(3 - support_count) * 1e6` (strongly prefers supported positions)
     - Position score: `Y * 1e4 + Z * 1e2 + X` (prefers lower, closer to front, closer to left)
     - Waste score: sum of unused space in each dimension (prefers tighter fit)
3. The best-scoring valid placement is selected (points are scored first, and the full placement check only runs for points that could beat the best one so far)
4. **Fix-point correction** applied (same as Pivot engine)
5. After placement, the point list is updated:
   - Points inside the newly placed item are removed
   - New points are generated from the item's corners and from **intersection points** where existing items' faces cross the new item's boundaries
   - All candidate points are **projected downward** (gravity) to the nearest supporting surface
   - Points inside existing items or with zero available space are pruned
   - `MaxSpace` is recalculated for all remaining points

### Key Features

- **Quick rejection** via `MaxSpace` avoids expensive collision detection for most candidates
- **Intersection points** between items create denser candidate positions, finding placements that corner-based engines miss
- **Gravity projection** ensures items settle onto surfaces rather than floating
- **Support counting** tracks how many planes (floor, walls, item surfaces) touch each point

### Configuration

| Option | Description |
|---|---|
| `WithEPStability(ratio)` | Enable stability checking with the given support ratio threshold. |

### Characteristics

- **Speed**: ~1.4ms for 50 items (1901 allocs, 386KB)
- **Quality**: Best -- intersection points and scoring produce the densest packing
- **Scaling**: O(n^2) point maintenance (each placement updates all points against all items). With TrialPacking, 800 items take about 2 s on an Apple M4 Pro (9 minutes before v0.3.0, which rebuilt the points every time the solver switched bins)
- **State**: Stateful -- keeps the point list of every bin it packs (see [Engine State](#engine-state))
- **Best for**: When packing quality is the top priority and you have <100 items

### Example

```go
engine := placement.NewExtremePointEngine(
    placement.WithEPStability(0.5),
)

p := packer.NewPacker(
    packer.WithPlacementEngine(engine),
)
```

## LAFF Engine (Largest Area Fit First)

```go
placement.NewLAFFEngine()                       // Full: allows stacking within levels
placement.NewLAFFEngine(placement.LAFFFast())    // Fast: 2D-only within levels
placement.NewLAFFEngine(placement.WithLAFFStability(0.7))
```

### Algorithm

The LAFF engine divides the bin into horizontal **levels** (shelves). There is one level per height at which items start, as tall as the tallest item starting there, so the levels follow from what the bin holds.

1. **First item**: creates a new level at Y=0. The level height equals the item's effective height in the chosen rotation. The rotation maximizing base area (width * depth) is preferred.
2. **Subsequent items**: try existing levels (highest first), then create a new level on top of everything placed so far if no existing level works
3. **Within a level**:
   - Candidate positions are generated from corners of items already on the level
   - 2D candidates: `(x + width, level_y, z)` and `(x, level_y, z + depth)` for each item on the level
   - **Full variant** additionally generates 3D stacking candidates at `(x, y + height, z)` within the level (intra-level stacking)
   - **Fast variant** (`LAFFFast()`) only uses 2D candidates -- items are laid flat on the level floor with no stacking within the level
4. Items must fit within the level's height
5. **Fix-point correction** applied after placement

### Variants

| Variant | Constructor | Within-level behavior |
|---|---|---|
| LAFF (full) | `NewLAFFEngine()` | 2D placement + stacking within levels |
| LAFF-Fast | `NewLAFFEngine(LAFFFast())` | 2D placement only (no intra-level stacking) |

### Configuration

| Option | Description |
|---|---|
| `LAFFFast()` | Enable 2D-only fast variant |
| `WithLAFFStability(ratio)` | Enable stability checking with the given support ratio threshold |

### Characteristics

| Variant | Speed | Allocs | Memory |
|---|---|---|---|
| LAFF-Fast | ~0.08ms | 390 | 128KB |
| LAFF (full) | ~0.08ms | 423 | 186KB |

- **Quality**: Good for uniform or shelf-like items. Less optimal for highly mixed sizes (level height is wasted when items are much shorter than the first item on the level).
- **Scaling**: O(n * levels) per item -- scales well to thousands of items
- **State**: Stateful -- keeps the levels of every bin it packs (see [Engine State](#engine-state))
- **Best for**: High-throughput scenarios (batch processing, real-time systems) and uniform item sets

### Example

```go
// Fast variant for maximum throughput
engine := placement.NewLAFFEngine(placement.LAFFFast())

p := packer.NewPacker(
    packer.WithPlacementEngine(engine),
)
```

## MaxRects Engine

```go
placement.NewMaxRectsEngine()
placement.NewMaxRectsEngine(placement.WithMaxRectsStability(0.7))
```

### Algorithm

The MaxRects engine keeps the list of **maximal free spaces** of the bin: the largest empty boxes that fit between the placed items and the walls.

1. **Candidates**: for each free space and each allowed rotation that fits in it, the item is placed at the space's corner and dropped onto the items under the space (gravity)
2. **Scoring** (lower is better): `Y * 1e10 + shortSide * 1e6 + Z * 100 + X` -- lowest position first, then the tightest fit (Best Short Side Fit)
3. **Validation**: spots are tried from the best score down until one passes the full placement check
4. **Update**: every free space the item overlaps is split into up to six sub-spaces around it, and sub-spaces contained in another space are dropped

### Configuration

| Option | Description |
|---|---|
| `WithMaxRectsStability(ratio)` | Enable stability checking with the given support ratio threshold. |

### Characteristics

- **Speed**: ~4ms for 50 items (675 allocs, 286KB)
- **Quality**: Close to Extreme Points. Before v0.3.0 the engine lost most of its free spaces when an item split several of them, and used 20% more boxes
- **State**: Stateful -- keeps the free spaces of every bin it packs (see [Engine State](#engine-state))
- **Best for**: Mixed item sizes where filling gaps matters

## Common Features

All engines share these behaviors:

### Fix-Point Correction

After finding a valid placement, all engines apply compaction. The item is pushed toward the origin along each axis:

1. **Y axis** (gravity): item slides down to the highest supporting surface below it
2. **X axis**: item slides left to the nearest item edge or wall
3. **Z axis**: item slides forward to the nearest item edge or wall

If the corrected position fails validation (e.g., stability checks), the original position is kept.

### Stability Checking

When enabled, items must have a minimum fraction of their base area resting on surfaces below:

```go
// Pivot engine
placement.NewPivotEngine(placement.WithStability(0.7))

// Extreme Points engine
placement.NewExtremePointEngine(placement.WithEPStability(0.7))

// LAFF engine
placement.NewLAFFEngine(placement.WithLAFFStability(0.7))
```

Items on the floor always have a support ratio of 1.0.

### Fragile Item Handling

Nothing may rest on a fragile item: a new item cannot be placed on top of one, and a fragile item cannot be slid under an item placed earlier.

### Load-Bearing Capacity

Load limits are always enforced, with or without stability, and count the full weight stacked on an item. Placing an item checks its own limit (for items it would be slid under) and the limits of every item under it. See [Physical Constraints](constraints.md#load-bearing-capacity).

### Weight Limits

Before placing an item, the engine verifies that `bin.CanCarry(item.Weight)`. A `MaxWeight` of 0 means no limit.

### Rotation

All engines try every allowed rotation for each candidate position. Default: all 6 rotations. Restricted via `ItemUpright()` or `ItemAllowedRotations()`.

### Engine State

Extreme Points, MaxRects and LAFF keep per-bin state (points, free spaces, levels) for every bin they pack, so a solver can switch between open bins without rebuilding it. A bin an engine has not packed itself is rebuilt from its items exactly as if the engine had placed them, so one engine switching between bins gives the same result as one engine per bin.

Engines are not safe for concurrent use: give each goroutine its own (solvers take engine factories for this). To reuse an engine for an unrelated packing run, call `Reset()`; `packer.Packer` does it on every `Pack`.

## Comparison Table

Times from the `BenchmarkPack50Items_*` benchmarks in `pkg/packer` on an AMD Ryzen 9 9950X3D (see [Performance](performance.md)).

| Feature | Pivot | Extreme Points | MaxRects | LAFF / LAFF-Fast |
|---|---|---|---|---|
| **Speed (50 items)** | ~0.1ms | ~1.4ms | ~4ms | ~0.08ms / ~0.08ms |
| **Memory** | 33KB | 386KB | 286KB | 186KB / 128KB |
| **Allocations** | 336 | 1901 | 675 | 423 / 390 |
| **Packing quality** | Good | Best | Close to best | Good (uniform items) |
| **Candidate generation** | 3 corners per item | Corners + intersections | Corners of free spaces | Level-based corners |
| **Quick rejection** | Recent blockers | MaxSpace, score first | Space size | Recent blockers |
| **Gravity projection** | Fix-point | Yes | Yes | Fix-point |
| **Scoring** | First valid | Multi-criteria best | Best short side fit | First valid |
| **State** | Stateless | Per bin | Per bin | Per bin |
| **Scaling** | O(n^2) | O(n^2) | O(n * spaces) | O(n * levels) |
| **Best for** | General purpose | Quality-critical | Mixed sizes | High throughput |

## Decision Guide

```
Is throughput the primary concern?
  YES -> LAFF or LAFF-Fast (~0.08ms for 50 items)
  NO  -> Is packing quality critical?
           YES -> ExtremePoints (best utilization, ~1.4ms) or MaxRects
           NO  -> Pivot (good balance, ~0.1ms) -- the default
```

If you are unsure, use the [Parallel solver](solvers.md) to try all engines concurrently and automatically pick the best result.
