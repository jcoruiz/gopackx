package stability_test

import (
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/stability"
)

func ExampleSupportRatio() {
	// A bottom item on the floor.
	bottom := model.NewItem("bottom", 40, 10, 40, 20)
	bottom.Position = [3]float64{0, 0, 0}
	bottom.Placed = true

	// An item on the floor has full support.
	ratio := stability.SupportRatio(bottom, nil)
	fmt.Printf("Floor item support: %.2f\n", ratio)

	// A top item perfectly aligned on the bottom item.
	top := model.NewItem("top", 40, 10, 40, 5)
	top.Position = [3]float64{0, 10, 0}
	top.Placed = true

	ratio = stability.SupportRatio(top, []*model.Item{bottom})
	fmt.Printf("Fully supported: %.2f\n", ratio)

	// A top item half overlapping the bottom item.
	half := model.NewItem("half", 40, 10, 40, 5)
	half.Position = [3]float64{20, 10, 0}
	half.Placed = true

	ratio = stability.SupportRatio(half, []*model.Item{bottom})
	fmt.Printf("Half supported: %.2f\n", ratio)

	// Output:
	// Floor item support: 1.00
	// Fully supported: 1.00
	// Half supported: 0.50
}

func ExampleGravityCenter() {
	// Single item in the front-left corner of a 100x100 bin.
	item := model.NewItem("box", 50, 20, 50, 10)
	item.Position = [3]float64{0, 0, 0}
	item.Placed = true

	gc := stability.GravityCenter([]*model.Item{item}, 100, 100)
	fmt.Printf("Q1(front-left)=%.0f%% Q2(front-right)=%.0f%% Q3(back-left)=%.0f%% Q4(back-right)=%.0f%%\n",
		gc[0], gc[1], gc[2], gc[3])

	// Two items balanced across the bin.
	left := model.NewItem("left", 50, 20, 100, 10)
	left.Position = [3]float64{0, 0, 0}
	left.Placed = true

	right := model.NewItem("right", 50, 20, 100, 10)
	right.Position = [3]float64{50, 0, 0}
	right.Placed = true

	gc = stability.GravityCenter([]*model.Item{left, right}, 100, 100)
	fmt.Printf("Balanced: Q1=%.0f%% Q2=%.0f%% Q3=%.0f%% Q4=%.0f%%\n",
		gc[0], gc[1], gc[2], gc[3])

	// Output:
	// Q1(front-left)=100% Q2(front-right)=0% Q3(back-left)=0% Q4(back-right)=0%
	// Balanced: Q1=25% Q2=25% Q3=25% Q4=25%
}
