package intersection_test

import (
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func ExampleIntersect() {
	a := model.NewItem("a", 10, 10, 10, 1)
	a.Position = [3]float64{0, 0, 0}
	a.RotationType = model.RotationWHD

	// Item b overlaps with a.
	b := model.NewItem("b", 10, 10, 10, 1)
	b.Position = [3]float64{5, 5, 5}
	b.RotationType = model.RotationWHD

	// Item c does not overlap with a.
	c := model.NewItem("c", 10, 10, 10, 1)
	c.Position = [3]float64{20, 0, 0}
	c.RotationType = model.RotationWHD

	fmt.Printf("a-b intersect: %v\n", intersection.Intersect(a, b))
	fmt.Printf("a-c intersect: %v\n", intersection.Intersect(a, c))

	// Output:
	// a-b intersect: true
	// a-c intersect: false
}

func ExampleRectIntersect() {
	a := model.NewItem("a", 10, 10, 10, 1)
	a.Position = [3]float64{0, 0, 0}
	a.RotationType = model.RotationWHD

	b := model.NewItem("b", 10, 10, 10, 1)
	b.Position = [3]float64{5, 0, 20}
	b.RotationType = model.RotationWHD

	// On the width-height plane they overlap (X: 0-10 vs 5-15).
	fmt.Printf("WH plane: %v\n", intersection.RectIntersect(a, b, model.WidthAxis, model.HeightAxis))
	// On the width-depth plane they don't overlap (Z: 0-10 vs 20-30).
	fmt.Printf("WD plane: %v\n", intersection.RectIntersect(a, b, model.WidthAxis, model.DepthAxis))

	// Output:
	// WH plane: true
	// WD plane: false
}
