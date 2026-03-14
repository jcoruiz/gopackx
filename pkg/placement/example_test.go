package placement_test

import (
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
)

func ExampleNewPivotEngine() {
	engine := placement.NewPivotEngine(
		placement.WithStability(0.7),
	)

	bin := model.NewBin("bin-1", 100, 100, 100, 500)
	item := model.NewItem("box-1", 50, 50, 50, 10)

	placed := engine.PlaceItem(bin, item)
	fmt.Printf("Placed: %v\n", placed)
	fmt.Printf("Position: %v\n", item.Position)
	fmt.Printf("Items in bin: %d\n", len(bin.Items))

	// Output:
	// Placed: true
	// Position: [0 0 0]
	// Items in bin: 1
}

func ExampleNewExtremePointEngine() {
	engine := placement.NewExtremePointEngine(
		placement.WithEPStability(0.8),
	)

	bin := model.NewBin("bin-1", 100, 100, 100, 500)
	a := model.NewItem("a", 50, 50, 50, 10)
	b := model.NewItem("b", 50, 50, 50, 10)

	engine.PlaceItem(bin, a)
	engine.PlaceItem(bin, b)

	fmt.Printf("Items in bin: %d\n", len(bin.Items))
	fmt.Printf("Item A position: %v\n", a.Position)
	fmt.Printf("Item B position: %v\n", b.Position)

	// Output:
	// Items in bin: 2
	// Item A position: [0 0 0]
	// Item B position: [50 0 0]
}

func ExampleNewLAFFEngine() {
	engine := placement.NewLAFFEngine(
		placement.WithLAFFStability(0.6),
		placement.LAFFFast(),
	)

	bin := model.NewBin("bin-1", 100, 100, 100, 500)
	a := model.NewItem("a", 60, 30, 60, 10)
	b := model.NewItem("b", 40, 30, 40, 8)

	engine.PlaceItem(bin, a)
	engine.PlaceItem(bin, b)

	fmt.Printf("Items in bin: %d\n", len(bin.Items))
	fmt.Printf("Item A position: %v\n", a.Position)

	// Output:
	// Items in bin: 2
	// Item A position: [0 0 0]
}

func ExampleNewMaxRectsEngine() {
	engine := placement.NewMaxRectsEngine(
		placement.WithMaxRectsStability(0.7),
	)

	bin := model.NewBin("bin-1", 100, 100, 100, 500)
	a := model.NewItem("a", 50, 50, 50, 10)
	b := model.NewItem("b", 50, 50, 50, 10)

	engine.PlaceItem(bin, a)
	engine.PlaceItem(bin, b)

	fmt.Printf("Items in bin: %d\n", len(bin.Items))
	fmt.Printf("Item A position: %v\n", a.Position)
	fmt.Printf("Item B position: %v\n", b.Position)

	// Output:
	// Items in bin: 2
	// Item A position: [0 0 0]
	// Item B position: [50 0 0]
}
