# GoPackX

A high-performance 3D bin packing library for Go. GoPackX solves the problem of fitting items into containers optimally, considering dimensions, weight, rotations, stability, and physical constraints.

## Features

- **Multiple placement engines** — Pivot Points, Extreme Points, and LAFF (Largest Area Fit First)
- **7 packing strategies** — BestFitDecreasing, MinimizeBins, BestFit, Greedy, NextFit, WorstFit, AlmostWorstFit
- **Advanced solvers** — Branch & Bound (exact) and Parallel multi-config solver
- **Physical constraints** — weight limits, load-bearing capacity, fragile items, stability checks, gravity center analysis
- **6 rotation types** with support for upright-only and custom rotation restrictions
- **Fix-point correction** — automatically compacts items toward the origin for tighter packing
- **Context-aware** — all operations respect `context.Context` for cancellation and deadlines
- **Zero dependencies** — only the Go standard library

## Installation

```bash
go get github.com/jcoruiz/gopackx
```

Requires Go 1.22 or later.

## Usage

### Basic packing

```go
package main

import (
	"context"
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
)

func main() {
	p := packer.NewPacker()

	p.AddBin(model.NewBin("container", 100, 100, 100, 500))

	p.AddItem(model.NewItem("box-1", 50, 50, 50, 10))
	p.AddItem(model.NewItem("box-2", 40, 40, 40, 8))
	p.AddItem(model.NewItem("box-3", 30, 30, 30, 5))

	result, err := p.Pack(context.Background())
	if err != nil {
		panic(err)
	}

	fmt.Printf("Fitted: %d/%d items\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Volume used: %.1f%%\n", result.Stats.VolumeUsedPct)
}
```

### Item options

```go
// Upright-only (no tipping)
item := model.NewItem("fragile-tv", 80, 50, 10, 15, model.ItemUpright())

// Fragile (nothing stacked on top)
item := model.NewItem("glass", 30, 20, 20, 5, model.ItemFragile())

// Load-bearing capacity (max 50kg on top)
item := model.NewItem("crate", 40, 40, 40, 20, model.ItemLoadBear(50))

// Priority (1 = highest, packed first)
item := model.NewItem("urgent", 20, 20, 20, 3, model.ItemPriority(1))

// Binding group (keeps related items together)
item := model.NewItem("set-a-1", 10, 10, 10, 1, model.ItemGroup("set-a"))
```

### Choosing a strategy

```go
import "github.com/jcoruiz/gopackx/pkg/strategy"

p := packer.NewPacker(packer.WithStrategy(strategy.MinimizeBins))
```

| Strategy | Description |
|---|---|
| `BestFitDecreasing` | Large items first, smallest remaining bin (default) |
| `MinimizeBins` | Large items first, first available bin |
| `BestFit` | Small items first, smallest remaining bin |
| `Greedy` | Small items first, first available bin |
| `NextFit` | Small items first, current bin only |
| `WorstFit` | Small items first, largest remaining bin |
| `AlmostWorstFit` | Small items first, largest bin skipping >80% empty |

### Choosing a placement engine

```go
import "github.com/jcoruiz/gopackx/pkg/placement"

// Pivot Points (default) — fast, corner-based candidates
p := packer.NewPacker(packer.WithPlacementEngine(placement.NewPivotEngine()))

// Extreme Points — scored placement with space/support metadata
p := packer.NewPacker(packer.WithPlacementEngine(placement.NewExtremePointEngine()))

// LAFF — level-based horizontal stacking
p := packer.NewPacker(packer.WithPlacementEngine(placement.NewLAFFEngine()))

// LAFF Fast — 2D-only within levels
p := packer.NewPacker(packer.WithPlacementEngine(placement.NewLAFFEngine(placement.LAFFFast())))
```

### Stability constraints

```go
// Pivot engine with 70% support ratio requirement
engine := placement.NewPivotEngine(placement.WithStability(0.7))

// Extreme Point engine with stability
engine := placement.NewExtremePointEngine(placement.WithEPStability(0.7))
```

### Branch & Bound solver

Exhaustively searches for the optimal item ordering within a time budget.

```go
import "github.com/jcoruiz/gopackx/pkg/solver"

bb := solver.NewBranchBound(func() placement.Engine {
	return placement.NewPivotEngine()
})

ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

result, err := bb.Solve(ctx, bins, items)
```

The full variant (`BBFull()`) also explores all rotation combinations per item — more thorough but slower.

### Parallel multi-config solver

Runs multiple engine/strategy combinations concurrently and returns the best result.

```go
ps := solver.NewParallel() // uses a default set of 5 configurations

// Or specify custom configurations:
ps := solver.NewParallel(
	solver.WithConfig(func() placement.Engine { return placement.NewPivotEngine() }, strategy.BestFitDecreasing),
	solver.WithConfig(func() placement.Engine { return placement.NewExtremePointEngine() }, strategy.MinimizeBins),
)

result, err := ps.Solve(ctx, bins, items)
```

## Architecture

```
pkg/
├── model/          Core types: Item, Bin, Result, PackingStats
├── packer/         Public API — orchestrates strategy + engine
├── placement/      Placement engines (Pivot, ExtremePoint, LAFF)
├── solver/         Advanced solvers (BranchBound, Parallel)
├── strategy/       Item sorting and bin selection strategies
├── rotation/       Rotation utilities and constraint checking
├── intersection/   AABB 3D collision detection
└── stability/      Support ratio, load-bearing, gravity center
```

## Testing

```bash
go test ./...
```

## License

[MIT](LICENSE)
