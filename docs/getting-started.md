# Getting Started with GoPackX

GoPackX is a zero-dependency 3D bin packing library for Go. It solves the problem of fitting rectangular items into rectangular containers (bins), optimizing for volume utilization and respecting physical constraints like weight, rotation, stability, and fragility.

## Prerequisites

- **Go 1.22** or later

## Installation

```bash
go get github.com/jcoruiz/gopackx
```

## Your First Program

This walkthrough creates a packer, adds a bin and some items, runs the packing algorithm, and reads the results.

### Step 1: Create a Packer

The `Packer` is the main entry point. With no options, it uses the **BestFitDecreasing** strategy and the **Pivot Points** placement engine -- good defaults for most workloads.

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/jcoruiz/gopackx/pkg/model"
    "github.com/jcoruiz/gopackx/pkg/packer"
)

func main() {
    p := packer.NewPacker()
```

### Step 2: Add Bins

A `Bin` is a container with width, height, depth, and a maximum weight capacity. You can add one or more bins -- the packer will distribute items across them.

```go
    bin := model.NewBin("warehouse-bin", 100, 100, 100, 500)
    p.AddBin(bin)
```

### Step 3: Add Items

An `Item` has an ID, width, height, depth, and weight. Optional settings (rotation restrictions, priority, fragility, etc.) are passed as functional options.

```go
    items := []*model.Item{
        model.NewItem("box-A", 50, 50, 50, 20),
        model.NewItem("box-B", 40, 30, 30, 15),
        model.NewItem("box-C", 30, 20, 20, 10),
        model.NewItem("box-D", 25, 25, 25, 12),
        model.NewItem("box-E", 60, 40, 40, 30),
        model.NewItem("box-F", 20, 20, 20, 5),
    }
    for _, item := range items {
        p.AddItem(item)
    }
```

### Step 4: Pack

Call `Pack` with a `context.Context`. All packing operations respect context cancellation and deadlines.

```go
    result, err := p.Pack(context.Background())
    if err != nil {
        log.Fatalf("packing failed: %v", err)
    }
```

### Step 5: Read the Results

The `Result` contains the packed bins, any items that did not fit, and summary statistics.

```go
    fmt.Printf("Fitted: %d / %d items\n", result.Stats.FittedItems, result.Stats.TotalItems)
    fmt.Printf("Volume utilization: %.1f%%\n", result.Stats.VolumeUsedPct)

    for _, b := range result.Bins {
        if len(b.Items) == 0 {
            continue
        }
        fmt.Printf("\nBin %q: %d items, %.1f%% volume used\n", b.ID, len(b.Items), b.VolumeUsedPct())
        for _, item := range b.Items {
            dim := item.Dimension()
            fmt.Printf("  %s at (%.1f, %.1f, %.1f) size %.0fx%.0fx%.0f rotation=%d\n",
                item.ID,
                item.Position[0], item.Position[1], item.Position[2],
                dim[0], dim[1], dim[2],
                item.RotationType)
        }
    }

    if len(result.UnfittedItems) > 0 {
        fmt.Println("\nDid not fit:")
        for _, item := range result.UnfittedItems {
            fmt.Printf("  %s (%.0fx%.0fx%.0f)\n", item.ID, item.Width, item.Height, item.Depth)
        }
    }
}
```

## Understanding the Result

### PackingStats

The `result.Stats` field contains summary metrics:

| Field | Description |
|---|---|
| `TotalBins` | Number of bins that received at least one item |
| `TotalItems` | Total number of items submitted |
| `FittedItems` | Number of items successfully placed |
| `UnfittedCount` | Number of items that could not be placed |
| `VolumeUsedPct` | Average volume utilization across active bins (0-100) |
| `WeightUsedPct` | Average weight utilization across active bins (0-100) |

### Accessing Placed Items

Each `Bin` in `result.Bins` has an `Items` slice with the placed items. Each placed item has:

- **`Position`** -- a `[3]float64` array with the `[X, Y, Z]` coordinates of the item's origin corner (front-bottom-left)
- **`RotationType`** -- which of the 6 rotation types was applied
- **`Dimension()`** -- returns the effective `[width, height, depth]` after rotation
- **`Placed`** -- `true` for items that were successfully placed

### Rotation Types

Items can be rotated into 6 orientations. Each rotation permutes the original `[W, H, D]` dimensions:

| Type | Name | Effective Dimensions |
|---|---|---|
| 0 | WHD | `[W, H, D]` (default) |
| 1 | HWD | `[H, W, D]` |
| 2 | HDW | `[H, D, W]` |
| 3 | DHW | `[D, H, W]` |
| 4 | DWH | `[D, W, H]` |
| 5 | WDH | `[W, D, H]` |

### Item Options

Items support functional options at creation time:

```go
// Keep upright (only rotations 0 and 3, preserving the height axis)
model.NewItem("fragile-tv", 60, 40, 10, 15, model.ItemUpright())

// Mark as fragile (nothing can be placed on top)
model.NewItem("glass", 30, 30, 30, 5, model.ItemFragile())

// Set packing priority (1 = highest priority, packed first)
model.NewItem("urgent", 20, 20, 20, 3, model.ItemPriority(1))

// Limit load-bearing capacity (max weight on top in kg)
model.NewItem("eggs", 40, 20, 30, 2, model.ItemLoadBear(5))

// Assign to a binding group (grouped items tend to land in the same bin)
model.NewItem("set-piece-1", 10, 10, 10, 1, model.ItemGroup("chess-set"))

// Restrict to specific rotations
model.NewItem("panel", 100, 50, 5, 10,
    model.ItemAllowedRotations([]model.RotationType{model.RotationWHD, model.RotationDHW}))
```

## Next Steps

- **[Strategies](strategies.md)** -- Learn about the 7 packing strategies and when to use each
- **[Placement Engines](placement-engines.md)** -- Understand Pivot Points, Extreme Points, and LAFF engines
- **[Solvers](solvers.md)** -- Use Branch & Bound and Parallel solvers for optimal results
- **[Constraints](constraints.md)** -- Configure weight, rotation, fragility, stability, and more
- **[Performance](performance.md)** -- Benchmarks, scaling considerations, and tuning tips
