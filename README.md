# GoPackX

A high-performance 3D bin packing library for Go with variable-sized box selection and cross-bin optimization. GoPackX solves the problem of fitting items into containers optimally, considering dimensions, weight, cost, rotations, stability, and physical constraints.

## Features

- **Variable-sized bin packing** — automatically selects which box types to use and how many, minimizing total boxes or total cost
- **Cross-bin optimization** — VNS metaheuristic redistributes items across bins to find solutions greedy approaches miss
- **Multiple placement engines** — Pivot Points, Extreme Points, MaxRects, and LAFF (Largest Area Fit First)
- **7 packing strategies** — BestFitDecreasing, MinimizeBins, BestFit, Greedy, NextFit, WorstFit, AlmostWorstFit
- **Advanced solvers** — TrialPacking, Metaheuristic, Branch & Bound (exact), and Parallel multi-config solver
- **Cost optimization** — assign costs per box type; solvers minimize total cost instead of box count
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

## Quick Start

The simplest way to use GoPackX: define your available box types and items, and let the library figure out the best combination.

```go
package main

import (
	"context"
	"fmt"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func main() {
	// Available box types (used as templates — solver creates instances as needed)
	boxes := []*model.Bin{
		model.NewBin("Small Box", 30, 25, 20, 5),
		model.NewBin("Medium Box", 40, 35, 30, 15),
		model.NewBin("Large Box", 60, 50, 40, 25),
	}

	items := []*model.Item{
		model.NewItem("item-1", 25, 20, 15, 2),
		model.NewItem("item-2", 35, 30, 25, 8),
		model.NewItem("item-3", 50, 40, 30, 12),
	}

	result, _ := gopackx.Pack(context.Background(), boxes, items)

	fmt.Printf("Fitted: %d/%d in %d boxes\n",
		result.Stats.FittedItems, result.Stats.TotalItems, result.Stats.TotalBins)
}
```

For better results at the cost of more computation, enable the metaheuristic optimizer:

```go
result, _ := gopackx.Pack(ctx, boxes, items, gopackx.Optimize())
```

### Cost optimization

When box types have different prices, the solver minimizes total cost instead of box count:

```go
boxes := []*model.Bin{
	model.NewBin("Small", 30, 25, 20, 5, model.BinCost(15.00)),
	model.NewBin("Large", 60, 50, 40, 25, model.BinCost(45.00)),
}

result, _ := gopackx.Pack(ctx, boxes, items)
fmt.Printf("Total cost: $%.2f\n", result.Stats.TotalCost)
```

## Usage

### Basic packing (fixed bins)

For packing into pre-created bins (when you already know which containers to use):

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

### Advanced solvers

```go
import "github.com/jcoruiz/gopackx/pkg/solver"

// TrialPacking — variable-sized bin packing with simulated bin selection
tp := solver.NewTrialPacking(engineFactory, solver.WithLookahead())
result, _ := tp.Solve(ctx, binTypes, items)

// Metaheuristic — cross-bin optimization via VNS
m := solver.NewMetaheuristic(engineFactory)
result, _ := m.Solve(ctx, binTypes, items)

// Branch & Bound — exhaustive per-bin optimization
bb := solver.NewBranchBound(engineFactory)
result, _ := bb.Solve(ctx, bins, items)

// Parallel — runs multiple engine/strategy combos concurrently
ps := solver.NewParallel()
result, _ := ps.Solve(ctx, bins, items)
```

See [Solvers documentation](docs/solvers.md) for details on each solver and when to use which.

## Architecture

```
gopackx.go          Top-level API: Pack(), Optimize(), WithEngine()
pkg/
├── model/          Core types: Item, Bin, Result, PackingStats
├── packer/         Simple packing into pre-created bins
├── placement/      Placement engines (Pivot, ExtremePoint, MaxRects, LAFF)
├── solver/         Advanced solvers (TrialPacking, Metaheuristic, BranchBound, Parallel)
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
