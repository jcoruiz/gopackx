package rotation_test

import (
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

func ExampleDimensions() {
	// Apply HWD rotation: height and width are swapped.
	dims := rotation.Dimensions(10, 20, 30, model.RotationHWD)
	fmt.Printf("HWD: %v\n", dims)

	// Apply DHW rotation: depth moves to width position.
	dims = rotation.Dimensions(10, 20, 30, model.RotationDHW)
	fmt.Printf("DHW: %v\n", dims)

	// Output:
	// HWD: [20 10 30]
	// DHW: [30 20 10]
}

func ExampleAllowedFor() {
	// Item with all rotations (default).
	all := model.NewItem("box", 10, 20, 30, 5)
	fmt.Printf("all rotations: %d\n", len(rotation.AllowedFor(all)))

	// Upright item: only 2 rotations.
	upright := model.NewItem("box", 10, 20, 30, 5, model.ItemUpright())
	fmt.Printf("upright rotations: %d\n", len(rotation.AllowedFor(upright)))

	// Output:
	// all rotations: 6
	// upright rotations: 2
}

func ExampleIsAllowed() {
	item := model.NewItem("box", 10, 20, 30, 5, model.ItemUpright())

	fmt.Printf("WHD allowed: %v\n", rotation.IsAllowed(item, model.RotationWHD))
	fmt.Printf("HWD allowed: %v\n", rotation.IsAllowed(item, model.RotationHWD))

	// Output:
	// WHD allowed: true
	// HWD allowed: false
}
