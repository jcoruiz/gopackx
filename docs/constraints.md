# Physical Constraints

GoPackX models real-world physical constraints that control how items can be placed in bins. Constraints are set on items (at creation time) and on engines (at configuration time). The placement validation layer enforces all constraints automatically.

## Weight Limits

Each bin has a maximum weight capacity. Each item has a weight. An item is only placed if the bin can accommodate its weight.

```go
// Bin with 500 kg capacity
bin := model.NewBin("truck", 200, 200, 200, 500)

// Items with weights
item1 := model.NewItem("heavy", 50, 50, 50, 300)  // 300 kg
item2 := model.NewItem("light", 30, 30, 30, 50)    // 50 kg
```

### How It Works

Before placing an item, the engine checks:

```
bin.RemainingWeight() >= item.Weight
```

Where `RemainingWeight()` returns `MaxWeight - TotalWeight()` (sum of all placed items' weights).

Items that exceed the remaining capacity are **skipped** (not placed) and appear in `result.UnfittedItems`.

### Post-Packing Queries

```go
bin.TotalWeight()      // sum of placed items' weights
bin.RemainingWeight()  // capacity left
bin.MaxWeight          // original capacity
```

## Rotation Restrictions

Items can be rotated into 6 orientations. Each rotation permutes the item's `[Width, Height, Depth]` dimensions.

### The 6 Rotation Types

| Type | Constant | Effective `[W, H, D]` | Description |
|---|---|---|---|
| 0 | `RotationWHD` | `[W, H, D]` | Default orientation (no rotation) |
| 1 | `RotationHWD` | `[H, W, D]` | Width and height swapped |
| 2 | `RotationHDW` | `[H, D, W]` | Height becomes width, depth becomes height |
| 3 | `RotationDHW` | `[D, H, W]` | Depth becomes width, original height preserved in middle |
| 4 | `RotationDWH` | `[D, W, H]` | Depth becomes width, width becomes height |
| 5 | `RotationWDH` | `[W, D, H]` | Width stays, depth and height swapped |

### Restricting Rotations

By default, all 6 rotations are allowed. You can restrict them:

**Upright only** -- only rotations that keep the height axis vertical (types 0 and 3):

```go
item := model.NewItem("tv", 60, 40, 10, 15, model.ItemUpright())
```

This allows `RotationWHD` (`[W, H, D]`) and `RotationDHW` (`[D, H, W]`) -- both preserve the original height in the Y axis.

**Custom rotation set**:

```go
item := model.NewItem("panel", 100, 50, 5, 10,
    model.ItemAllowedRotations([]model.RotationType{
        model.RotationWHD,   // [100, 50, 5]
        model.RotationWDH,   // [100, 5, 50]
    }),
)
```

### How It Works

During placement, the engine iterates over `item.AllowedRotations` and tries each rotation at each candidate position. The first valid rotation+position combination is used.

## Fragile Items

Fragile items cannot have anything placed on top of them.

```go
item := model.NewItem("glass-panel", 50, 50, 5, 3, model.ItemFragile())
```

### How It Works

When the engine tries to place a new item, it checks all already-placed fragile items. If the new item would rest on top of a fragile item (detected via XZ-plane overlap at the fragile item's top Y position), the placement is rejected.

Specifically, for each placed item with `Fragile == true`:

1. Calculate the fragile item's top Y: `position.Y + dimension.Height`
2. Check if the new item's bottom Y matches the fragile item's top Y (within epsilon)
3. Check if there is overlap in both the width (X) and depth (Z) axes
4. If all three conditions are met, the placement fails

Fragile items can still have items placed **beside** them (same Y level or below). Only items **on top** are blocked.

## Load-Bearing Capacity

Items can have a maximum weight they can support on top.

```go
// This item can support up to 10 kg on top
item := model.NewItem("cardboard-box", 40, 30, 30, 5, model.ItemLoadBear(10))
```

### How It Works

Load-bearing is checked **only when stability is enabled** on the engine:

```go
engine := placement.NewPivotEngine(placement.WithStability(0.7))
```

When a new item is placed, the engine temporarily adds it to the bin, then verifies that no item below has its load-bearing capacity exceeded.

The weight calculation uses **proportional distribution** based on overlap area:

```
weight_on_item = sum over items_above of (item_above.Weight * overlap_area / item_above.base_area)
```

Where `overlap_area` is the intersection of the XZ projections of the two items, and the above item must be resting directly on top (its bottom Y equals the below item's top Y).

If `LoadBear` is 0 (default), no load limit is enforced. If `Fragile` is true, *any* weight on top causes failure (equivalent to `LoadBear(0)` with strict enforcement).

### Example: Stacking Constraints

```go
engine := placement.NewPivotEngine(placement.WithStability(0.5))

p := packer.NewPacker(packer.WithPlacementEngine(engine))

bin := model.NewBin("bin-1", 100, 100, 100, 500)
p.AddBin(bin)

// Strong base: can hold 100 kg
p.AddItem(model.NewItem("crate", 50, 20, 50, 30, model.ItemLoadBear(100)))

// Medium box: can hold 20 kg
p.AddItem(model.NewItem("box", 40, 15, 40, 10, model.ItemLoadBear(20)))

// Light fragile: nothing on top
p.AddItem(model.NewItem("vase", 20, 30, 20, 2, model.ItemFragile()))
```

## Support Ratio (Stability)

The support ratio measures what fraction of an item's base area rests on surfaces below it. This prevents items from being placed in positions where they would "float" or overhang.

### Enabling Stability

Stability is configured **per engine** (it is not enabled by default):

```go
// Pivot engine with 70% support required
placement.NewPivotEngine(placement.WithStability(0.7))

// Extreme Points engine with 50% support
placement.NewExtremePointEngine(placement.WithEPStability(0.5))

// LAFF engine with 90% support
placement.NewLAFFEngine(placement.WithLAFFStability(0.9))
```

### How It Works

For each item being placed:

1. If the item is on the floor (Y position ~= 0), its support ratio is **1.0** (fully supported)
2. Otherwise, find all items directly below it (their top Y equals this item's bottom Y)
3. Calculate the XZ overlap area between this item and each supporting item
4. Support ratio = `total_overlap_area / item_base_area`

The item is placed only if `support_ratio >= threshold`.

### Choosing a Ratio

| Ratio | Effect | Use Case |
|---|---|---|
| 0.0 | Disabled (no stability check) | Maximum packing density, ignore physics |
| 0.5 | Relaxed -- item needs half its base supported | General packing with some stability |
| 0.7 | Balanced -- good trade-off between density and stability | Recommended starting point |
| 0.9 | Strict -- nearly full support required | Fragile or heavy items |
| 1.0 | Full support -- entire base must rest on something | Very strict physical simulation |

Higher ratios reduce packing density because fewer positions are valid. Start with 0.7 and adjust based on your needs.

### Post-Packing Analysis

You can also check support ratios after packing:

```go
import "github.com/jcoruiz/gopackx/pkg/stability"

for _, item := range bin.Items {
    ratio := stability.SupportRatio(item, bin.Items)
    fmt.Printf("%s: %.0f%% supported\n", item.ID, ratio*100)
}
```

## Priority

Priority controls the order in which items are packed. Lower numbers mean higher priority.

```go
// Priority 1 = highest, packed first
urgent := model.NewItem("urgent", 20, 20, 20, 5, model.ItemPriority(1))
normal := model.NewItem("normal", 20, 20, 20, 5, model.ItemPriority(5))
low    := model.NewItem("low",    20, 20, 20, 5, model.ItemPriority(10))
```

### How It Works

Priority is the **third tiebreaker** in item sorting (after volume and load-bearing capacity). Within the same volume and load-bearing tier, items with lower priority numbers are packed first.

This means priority interacts with the strategy's volume sort:

- In **BestFitDecreasing** (volume descending), a small item with priority 1 may still be packed *after* a large item with priority 10, because volume sorting comes first
- Priority is most impactful when items have similar volumes

Priority 0 (default) is treated as "no priority set" and sorts after items with explicit priorities.

## Binding Groups

Binding groups increase the chance that related items end up in the same bin.

```go
piece1 := model.NewItem("leg-1", 10, 50, 10, 2, model.ItemGroup("table"))
piece2 := model.NewItem("leg-2", 10, 50, 10, 2, model.ItemGroup("table"))
piece3 := model.NewItem("top",   60, 5, 40, 8,  model.ItemGroup("table"))
```

### How It Works

Group is the **fourth tiebreaker** in item sorting (after volume, load-bearing, and priority). Items with the same group name are sorted adjacently in alphabetical order. Since items are packed in order, adjacent items are more likely to land in the same bin.

This is a **soft constraint** -- it influences ordering but does not guarantee co-placement. If items in a group don't fit in the same bin, they will be distributed across bins normally.

## Gravity Center

The gravity center analysis calculates how weight is distributed across the bin's floor area, divided into four quadrants.

```go
import "github.com/jcoruiz/gopackx/pkg/stability"

gc := stability.GravityCenter(bin.Items, bin.Width, bin.Depth)
fmt.Printf("Q1 (front-left):  %.1f%%\n", gc[0])
fmt.Printf("Q2 (front-right): %.1f%%\n", gc[1])
fmt.Printf("Q3 (back-left):   %.1f%%\n", gc[2])
fmt.Printf("Q4 (back-right):  %.1f%%\n", gc[3])
```

### Quadrant Layout

Looking at the bin from above:

```
         Width (X) -->
        +----------+----------+
        |          |          |
 Depth  |  Q1      |  Q2      |
 (Z)    | front-   | front-   |
  |     | left     | right    |
  v     +----------+----------+
        |          |          |
        |  Q3      |  Q4      |
        | back-    | back-    |
        | left     | right    |
        +----------+----------+
```

### Return Value

Returns `[4]float64` with percentages (0-100) that sum to 100:

- `gc[0]` = Q1 (front-left)
- `gc[1]` = Q2 (front-right)
- `gc[2]` = Q3 (back-left)
- `gc[3]` = Q4 (back-right)

A perfectly balanced load has all quadrants at **25%**. The function distributes each item's weight proportionally based on how much of its XZ footprint overlaps each quadrant.

### When to Use

Gravity center is a **post-packing analysis tool** -- it is not enforced during placement. Use it to:

- Verify load balance after packing (e.g., for shipping containers)
- Flag bins with uneven weight distribution
- Compare packing configurations

### Example: Checking Balance

```go
result, _ := p.Pack(ctx)

for _, bin := range result.Bins {
    if len(bin.Items) == 0 {
        continue
    }
    gc := stability.GravityCenter(bin.Items, bin.Width, bin.Depth)
    maxDelta := 0.0
    for _, q := range gc {
        delta := math.Abs(q - 25.0)
        if delta > maxDelta {
            maxDelta = delta
        }
    }
    if maxDelta > 10 {
        fmt.Printf("WARNING: Bin %s has unbalanced weight (max deviation: %.1f%%)\n", bin.ID, maxDelta)
    }
}
```

## Constraint Interaction

Constraints are checked in this order during placement validation:

1. **Bin boundaries** -- item must fit within bin dimensions
2. **Positive position** -- no negative coordinates
3. **Weight capacity** -- bin has enough remaining weight
4. **Intersection** -- no overlap with already-placed items
5. **Stability** -- support ratio met (if stability enabled)
6. **Fragile** -- not placed on top of any fragile item
7. **Load-bearing** -- items below not overloaded (if stability enabled)

If any check fails, the placement is rejected and the engine tries the next candidate position or rotation.
