# Placement Engines

A placement engine determines **where** inside a bin each item is physically positioned. GoPackX provides three engines, each implementing the `placement.Engine` interface:

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

- **Speed**: ~1.5ms for 50 items (1991 allocs, 192KB)
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
3. The best-scoring valid placement is selected
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

- **Speed**: ~17ms for 50 items (1827 allocs, 304KB)
- **Quality**: Best -- intersection points and scoring produce the densest packing
- **Scaling**: O(n^2) point maintenance (each placement updates all points against all items)
- **State**: Stateful -- maintains point list per bin (reinitializes when bin changes)
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

The LAFF engine divides the bin into horizontal **levels** (shelves). Each level has a Y position and a height defined by the first item placed on it.

1. **First item**: creates a new level at Y=0. The level height equals the item's effective height in the chosen rotation. The rotation maximizing base area (width * depth) is preferred.
2. **Subsequent items**: try existing levels (most recent first), then create a new level if no existing level works
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
| LAFF-Fast | ~0.25ms | 377 | 115KB |
| LAFF (full) | ~0.38ms | 400 | 170KB |

- **Quality**: Good for uniform or shelf-like items. Less optimal for highly mixed sizes (level height is wasted when items are much shorter than the first item on the level).
- **Scaling**: O(n * levels) per item -- scales well to thousands of items
- **State**: Stateful -- tracks levels per bin (reinitializes when bin changes)
- **Best for**: High-throughput scenarios (batch processing, real-time systems) and uniform item sets

### Example

```go
// Fast variant for maximum throughput
engine := placement.NewLAFFEngine(placement.LAFFFast())

p := packer.NewPacker(
    packer.WithPlacementEngine(engine),
)
```

## Common Features

All three engines share these behaviors:

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

If a fragile item is already placed, no new item can be placed on top of it. This is checked via XZ-plane overlap at the fragile item's top Y position.

### Load-Bearing Capacity

When stability is enabled, placing a new item checks that it does not overload any item below. Weight from above is distributed proportionally based on overlap area.

### Weight Limits

Before placing an item, the engine verifies that `bin.RemainingWeight() >= item.Weight`.

### Rotation

All engines try every allowed rotation for each candidate position. Default: all 6 rotations. Restricted via `ItemUpright()` or `ItemAllowedRotations()`.

## Comparison Table

| Feature | Pivot | Extreme Points | LAFF / LAFF-Fast |
|---|---|---|---|
| **Speed (50 items)** | ~1.5ms | ~17ms | ~0.38ms / ~0.25ms |
| **Memory** | 192KB | 304KB | 170KB / 115KB |
| **Allocations** | 1991 | 1827 | 400 / 377 |
| **Packing quality** | Good | Best | Good (uniform items) |
| **Candidate generation** | 3 corners per item | Corners + intersections | Level-based corners |
| **Quick rejection** | No | Yes (MaxSpace) | No |
| **Gravity projection** | No | Yes | No |
| **Scoring** | First valid | Multi-criteria best | First valid |
| **State** | Stateless | Stateful (per bin) | Stateful (per bin) |
| **Scaling** | O(n^2) | O(n^2) | O(n * levels) |
| **Best for** | General purpose | Quality-critical | High throughput |

## Decision Guide

```
Is throughput the primary concern?
  YES -> LAFF-Fast (~0.25ms, 4x faster than Pivot)
  NO  -> Is packing quality critical?
           YES -> ExtremePoints (best utilization, ~17ms)
           NO  -> Pivot (good balance, ~1.5ms) -- the default
```

If you are unsure, use the [Parallel solver](solvers.md) to try all engines concurrently and automatically pick the best result.
