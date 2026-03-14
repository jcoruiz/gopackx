// Example: physical constraints
//
// Demonstrates how gopackx handles various physical constraints:
//   - Upright-only items (e.g., a TV that cannot be tipped)
//   - Fragile items (nothing can be placed on top)
//   - Load-bearing capacity (maximum weight on top)
//   - Priority (higher priority items packed first)
//   - Binding groups (related items kept together)
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

func main() {
	fmt.Println("=== Physical Constraints Demo ===")
	fmt.Println()

	// --- 1. Upright-only items ---
	demoUpright()

	// --- 2. Fragile items ---
	demoFragile()

	// --- 3. Load-bearing capacity ---
	demoLoadBearing()

	// --- 4. Priority ---
	demoPriority()

	// --- 5. Binding groups ---
	demoGroups()
}

func demoUpright() {
	fmt.Println("--- 1. Upright-Only Items ---")
	fmt.Println("A TV (20x60x10) must keep its height axis vertical.")
	fmt.Println()

	// Create an upright-only item (e.g., a flat-screen TV).
	tv := model.NewItem("TV", 20, 60, 10, 8, model.ItemUpright())

	// Show which rotations are allowed.
	allowed := rotation.AllowedFor(tv)
	fmt.Printf("  Allowed rotations for %q: ", tv.ID)
	for i, r := range allowed {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(r)
	}
	fmt.Println()

	// Pack it.
	p := packer.NewPacker()
	p.AddBin(model.NewBin("shelf", 100, 100, 100, 200))
	p.AddItem(tv)
	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range result.Bins {
		for _, item := range b.Items {
			dim := item.Dimension()
			fmt.Printf("  Placed: %s at (%.0f, %.0f, %.0f) size %.0fx%.0fx%.0f rotation=%d\n",
				item.ID, item.Position[0], item.Position[1], item.Position[2],
				dim[0], dim[1], dim[2], item.RotationType)
			fmt.Printf("  Height axis remains vertical: effective height = %.0f (original = %.0f)\n",
				dim[1], tv.Height)
		}
	}
	fmt.Println()
}

func demoFragile() {
	fmt.Println("--- 2. Fragile Items ---")
	fmt.Println("A glass panel is fragile: nothing can be placed on top of it.")
	fmt.Println()

	// Use stability-aware engine to enforce placement rules.
	engine := placement.NewPivotEngine(placement.WithStability(0.5))
	p := packer.NewPacker(packer.WithPlacementEngine(engine))
	p.AddBin(model.NewBin("crate", 100, 100, 100, 300))

	// Fragile item placed first (high priority).
	glass := model.NewItem("glass-panel", 80, 5, 80, 3, model.ItemFragile(), model.ItemPriority(1))
	// Heavy box that would normally go on top.
	heavyBox := model.NewItem("heavy-box", 60, 40, 60, 50)

	p.AddItem(glass)
	p.AddItem(heavyBox)

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range result.Bins {
		fmt.Printf("  Bin %q: %d items packed\n", b.ID, len(b.Items))
		for _, item := range b.Items {
			dim := item.Dimension()
			fragileTag := ""
			if item.Fragile {
				fragileTag = " [FRAGILE]"
			}
			fmt.Printf("    %s at (%.0f, %.0f, %.0f) size %.0fx%.0fx%.0f%s\n",
				item.ID, item.Position[0], item.Position[1], item.Position[2],
				dim[0], dim[1], dim[2], fragileTag)
		}
	}
	fmt.Printf("  Unfitted: %d items\n", result.Stats.UnfittedCount)
	fmt.Println("  (Items cannot be stacked on top of the fragile glass panel)")
	fmt.Println()
}

func demoLoadBearing() {
	fmt.Println("--- 3. Load-Bearing Capacity ---")
	fmt.Println("A cardboard box can only support 10 kg on top.")
	fmt.Println()

	engine := placement.NewPivotEngine(placement.WithStability(0.5))
	p := packer.NewPacker(packer.WithPlacementEngine(engine))
	p.AddBin(model.NewBin("pallet", 100, 100, 100, 500))

	// Cardboard box with 10 kg load bearing limit.
	cardboard := model.NewItem("cardboard", 50, 30, 50, 5, model.ItemLoadBear(10), model.ItemPriority(1))
	// Light item that fits within the limit.
	lightItem := model.NewItem("light-pkg", 40, 20, 40, 8)
	// Heavy item that would exceed the limit.
	heavyItem := model.NewItem("heavy-pkg", 40, 20, 40, 25)

	p.AddItem(cardboard)
	p.AddItem(lightItem)
	p.AddItem(heavyItem)

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range result.Bins {
		fmt.Printf("  Bin %q: %d items packed\n", b.ID, len(b.Items))
		for _, item := range b.Items {
			dim := item.Dimension()
			lbTag := ""
			if item.LoadBear > 0 {
				lbTag = fmt.Sprintf(" [max %.0f kg on top]", item.LoadBear)
			}
			fmt.Printf("    %s (%.1f kg) at (%.0f, %.0f, %.0f) size %.0fx%.0fx%.0f%s\n",
				item.ID, item.Weight,
				item.Position[0], item.Position[1], item.Position[2],
				dim[0], dim[1], dim[2], lbTag)
		}
	}
	if len(result.UnfittedItems) > 0 {
		fmt.Printf("  Unfitted: ")
		for i, item := range result.UnfittedItems {
			if i > 0 {
				fmt.Print(", ")
			}
			fmt.Printf("%s (%.1f kg)", item.ID, item.Weight)
		}
		fmt.Println()
	}
	fmt.Println()
}

func demoPriority() {
	fmt.Println("--- 4. Priority ---")
	fmt.Println("Items with lower priority numbers are packed first.")
	fmt.Println()

	p := packer.NewPacker()
	// Small bin to force some items out.
	p.AddBin(model.NewBin("small-box", 50, 50, 50, 200))

	// Items with different priorities.
	p.AddItem(model.NewItem("urgent-A", 30, 30, 30, 10, model.ItemPriority(1)))
	p.AddItem(model.NewItem("urgent-B", 30, 30, 30, 10, model.ItemPriority(1)))
	p.AddItem(model.NewItem("normal-C", 30, 30, 30, 10, model.ItemPriority(5)))
	p.AddItem(model.NewItem("low-D", 30, 30, 30, 10, model.ItemPriority(10)))

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range result.Bins {
		fmt.Printf("  Bin %q: %d items packed\n", b.ID, len(b.Items))
		for _, item := range b.Items {
			fmt.Printf("    %s (priority=%d) placed=%v\n", item.ID, item.Priority, item.Placed)
		}
	}
	if len(result.UnfittedItems) > 0 {
		fmt.Println("  Unfitted (lower priority items excluded):")
		for _, item := range result.UnfittedItems {
			fmt.Printf("    %s (priority=%d)\n", item.ID, item.Priority)
		}
	}
	fmt.Println()
}

func demoGroups() {
	fmt.Println("--- 5. Binding Groups ---")
	fmt.Println("Items in the same group are kept together during sorting.")
	fmt.Println()

	p := packer.NewPacker()
	p.AddBin(model.NewBin("warehouse", 200, 200, 200, 1000))

	// Two groups of related items.
	p.AddItem(model.NewItem("electronics-1", 30, 20, 20, 5, model.ItemGroup("electronics")))
	p.AddItem(model.NewItem("electronics-2", 25, 15, 15, 3, model.ItemGroup("electronics")))
	p.AddItem(model.NewItem("food-1", 40, 30, 30, 10, model.ItemGroup("food")))
	p.AddItem(model.NewItem("food-2", 35, 25, 25, 8, model.ItemGroup("food")))
	// Items without a group.
	p.AddItem(model.NewItem("misc-1", 20, 20, 20, 4))
	p.AddItem(model.NewItem("misc-2", 15, 15, 15, 2))

	result, err := p.Pack(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	for _, b := range result.Bins {
		fmt.Printf("  Bin %q: %d items packed\n", b.ID, len(b.Items))
		for _, item := range b.Items {
			group := item.Group
			if group == "" {
				group = "(none)"
			}
			dim := item.Dimension()
			fmt.Printf("    %s  group=%-15s at (%.0f, %.0f, %.0f) size %.0fx%.0fx%.0f\n",
				item.ID, group,
				item.Position[0], item.Position[1], item.Position[2],
				dim[0], dim[1], dim[2])
		}
	}
	fmt.Println()
}
