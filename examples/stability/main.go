// Example: stability features
//
// Demonstrates stability analysis tools:
//   - Support ratio checking (items need surface support underneath)
//   - Load-bearing validation (items have max weight limits on top)
//   - Gravity center analysis (weight distribution across bin quadrants)
//   - How enabling stability affects placement decisions
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/stability"
)

func main() {
	fmt.Println("=== Stability Features ===")
	fmt.Println()

	// --- 1. Support ratio checking ---
	demoSupportRatio()

	// --- 2. Load-bearing validation ---
	demoLoadBearing()

	// --- 3. Gravity center analysis ---
	demoGravityCenter()

	// --- 4. Stability-aware vs. default packing ---
	demoStabilityComparison()
}

func demoSupportRatio() {
	fmt.Println("--- 1. Support Ratio Checking ---")
	fmt.Println("Checks what fraction of an item's base is supported by items below.")
	fmt.Println()

	// Simulate a placement scenario manually.
	// A large base item on the floor.
	base := model.NewItem("base", 50, 20, 50, 30)
	base.Position = [3]float64{0, 0, 0}
	base.RotationType = model.RotationWHD
	base.Placed = true

	// An item fully supported (sitting entirely on top of the base).
	fullySupported := model.NewItem("full-support", 30, 15, 30, 10)
	fullySupported.Position = [3]float64{5, 20, 5}
	fullySupported.RotationType = model.RotationWHD

	// An item partially supported (half hanging off the edge).
	partiallySupported := model.NewItem("partial-support", 40, 15, 40, 10)
	partiallySupported.Position = [3]float64{30, 20, 30}
	partiallySupported.RotationType = model.RotationWHD

	// An item in the air with no support.
	noSupport := model.NewItem("no-support", 20, 10, 20, 5)
	noSupport.Position = [3]float64{0, 60, 0}
	noSupport.RotationType = model.RotationWHD

	placed := []*model.Item{base}

	testItems := []*model.Item{fullySupported, partiallySupported, noSupport}
	for _, item := range testItems {
		ratio := stability.SupportRatio(item, placed)
		ok := stability.CheckSupport(item, placed, 0.5)
		fmt.Printf("  %-18s support ratio: %.2f  passes 50%% check: %v\n",
			item.ID, ratio, ok)
	}

	// Floor item always has ratio 1.0.
	floorRatio := stability.SupportRatio(base, nil)
	fmt.Printf("  %-18s support ratio: %.2f  (floor item, always 1.0)\n", base.ID, floorRatio)
	fmt.Println()
}

func demoLoadBearing() {
	fmt.Println("--- 2. Load-Bearing Validation ---")
	fmt.Println("Checks if the weight on top of an item exceeds its capacity.")
	fmt.Println()

	// A sturdy box with 50 kg load-bearing capacity.
	sturdy := model.NewItem("sturdy-box", 50, 30, 50, 10, model.ItemLoadBear(50))
	sturdy.Position = [3]float64{0, 0, 0}
	sturdy.RotationType = model.RotationWHD
	sturdy.Placed = true

	// A weak box with only 5 kg capacity.
	weak := model.NewItem("weak-box", 50, 20, 50, 5, model.ItemLoadBear(5))
	weak.Position = [3]float64{0, 30, 0}
	weak.RotationType = model.RotationWHD
	weak.Placed = true

	// A heavy item on top.
	heavy := model.NewItem("heavy-load", 40, 15, 40, 20)
	heavy.Position = [3]float64{5, 50, 5}
	heavy.RotationType = model.RotationWHD
	heavy.Placed = true

	allPlaced := []*model.Item{sturdy, weak, heavy}

	// Check weight above each item.
	for _, item := range []*model.Item{sturdy, weak} {
		weightAbove := stability.WeightAbove(item, allPlaced)
		ok := stability.CheckLoadBearing(item, allPlaced)
		fmt.Printf("  %-12s  capacity: %5.1f kg  weight above: %5.1f kg  ok: %v\n",
			item.ID, item.LoadBear, weightAbove, ok)
	}

	// Check a fragile item.
	fragile := model.NewItem("fragile-item", 50, 10, 50, 2, model.ItemFragile())
	fragile.Position = [3]float64{0, 0, 0}
	fragile.RotationType = model.RotationWHD
	fragile.Placed = true

	lightOnTop := model.NewItem("light-on-top", 20, 10, 20, 1)
	lightOnTop.Position = [3]float64{10, 10, 10}
	lightOnTop.RotationType = model.RotationWHD
	lightOnTop.Placed = true

	fragileItems := []*model.Item{fragile, lightOnTop}
	fragileOk := stability.CheckLoadBearing(fragile, fragileItems)
	fragileWeight := stability.WeightAbove(fragile, fragileItems)
	fmt.Printf("  %-12s  fragile=true       weight above: %5.1f kg  ok: %v\n",
		fragile.ID, fragileWeight, fragileOk)
	fmt.Println()
}

func demoGravityCenter() {
	fmt.Println("--- 3. Gravity Center Analysis ---")
	fmt.Println("Shows weight distribution across 4 quadrants of the bin floor.")
	fmt.Println("Q1=front-left, Q2=front-right, Q3=back-left, Q4=back-right")
	fmt.Println()

	binWidth, binDepth := 100.0, 100.0

	// Scenario A: weight concentrated in one corner.
	heavyCorner := model.NewItem("corner-load", 30, 20, 30, 100)
	heavyCorner.Position = [3]float64{0, 0, 0}
	heavyCorner.RotationType = model.RotationWHD

	gc := stability.GravityCenter([]*model.Item{heavyCorner}, binWidth, binDepth)
	fmt.Println("  Scenario A: Heavy item in front-left corner")
	printQuadrants(gc)

	// Scenario B: weight evenly distributed.
	items := []*model.Item{
		makePositioned("FL", 40, 20, 40, 25, 0, 0, 0),
		makePositioned("FR", 40, 20, 40, 25, 60, 0, 0),
		makePositioned("BL", 40, 20, 40, 25, 0, 0, 60),
		makePositioned("BR", 40, 20, 40, 25, 25, 60, 0),
	}

	gc = stability.GravityCenter(items, binWidth, binDepth)
	fmt.Println("  Scenario B: Weight in all four corners")
	printQuadrants(gc)

	// Scenario C: weight centered.
	centered := makePositioned("center", 40, 20, 40, 100, 30, 0, 30)
	gc = stability.GravityCenter([]*model.Item{centered}, binWidth, binDepth)
	fmt.Println("  Scenario C: Weight centered in the bin")
	printQuadrants(gc)
}

func demoStabilityComparison() {
	fmt.Println("--- 4. Stability-Aware vs. Default Packing ---")
	fmt.Println("Shows how enabling stability checking affects placement decisions.")
	fmt.Println()

	// Pack without stability.
	resultDefault := packWithStability(0)
	fmt.Println("  Without stability checking:")
	fmt.Printf("    Fitted: %d / %d\n", resultDefault.Stats.FittedItems, resultDefault.Stats.TotalItems)
	fmt.Printf("    Volume: %.1f%%\n", resultDefault.Stats.VolumeUsedPct)

	// Pack with 50% support ratio.
	result50 := packWithStability(0.5)
	fmt.Println("  With 50% support ratio:")
	fmt.Printf("    Fitted: %d / %d\n", result50.Stats.FittedItems, result50.Stats.TotalItems)
	fmt.Printf("    Volume: %.1f%%\n", result50.Stats.VolumeUsedPct)

	// Pack with 80% support ratio (stricter).
	result80 := packWithStability(0.8)
	fmt.Println("  With 80% support ratio (stricter):")
	fmt.Printf("    Fitted: %d / %d\n", result80.Stats.FittedItems, result80.Stats.TotalItems)
	fmt.Printf("    Volume: %.1f%%\n", result80.Stats.VolumeUsedPct)

	fmt.Println()
	fmt.Println("  Higher support ratio = more stable placements but potentially fewer items fit.")
}

// packWithStability runs a packing with the given support ratio.
// If ratio is 0, no stability checking is done.
func packWithStability(ratio float64) *model.Result {
	var engine placement.Engine
	if ratio > 0 {
		engine = placement.NewPivotEngine(placement.WithStability(ratio))
	} else {
		engine = placement.NewPivotEngine()
	}

	p := packer.NewPacker(packer.WithPlacementEngine(engine))
	p.AddBin(model.NewBin("container", 80, 80, 80, 500))

	items := []struct {
		id          string
		w, h, d, wt float64
	}{
		{"item-01", 30, 25, 20, 10},
		{"item-02", 35, 30, 25, 15},
		{"item-03", 20, 15, 10, 5},
		{"item-04", 25, 20, 15, 8},
		{"item-05", 40, 35, 30, 20},
		{"item-06", 15, 10, 10, 3},
		{"item-07", 50, 40, 35, 25},
		{"item-08", 20, 20, 20, 7},
		{"item-09", 30, 30, 25, 12},
		{"item-10", 35, 25, 20, 14},
	}
	for _, it := range items {
		p.AddItem(model.NewItem(it.id, it.w, it.h, it.d, it.wt))
	}

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatalf("packing failed: %v", err)
	}
	return result
}

func makePositioned(id string, w, h, d, weight, x, y, z float64) *model.Item {
	item := model.NewItem(id, w, h, d, weight)
	item.Position = [3]float64{x, y, z}
	item.RotationType = model.RotationWHD
	item.Placed = true
	return item
}

func printQuadrants(gc [4]float64) {
	fmt.Printf("    Q1 (front-left):  %5.1f%%\n", gc[0])
	fmt.Printf("    Q2 (front-right): %5.1f%%\n", gc[1])
	fmt.Printf("    Q3 (back-left):   %5.1f%%\n", gc[2])
	fmt.Printf("    Q4 (back-right):  %5.1f%%\n", gc[3])
	fmt.Println()
}
