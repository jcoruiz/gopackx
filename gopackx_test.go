package gopackx_test

import (
	"context"
	"fmt"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func ExamplePack() {
	boxTypes := []*model.Bin{
		model.NewBin("small", 30, 25, 20, 5),
		model.NewBin("large", 60, 50, 40, 25),
	}
	items := []*model.Item{
		model.NewItem("a", 25, 20, 15, 2),
		model.NewItem("b", 50, 40, 30, 10),
		model.NewItem("c", 20, 15, 10, 1),
	}

	result, err := gopackx.Pack(context.Background(), boxTypes, items)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Fitted: %d/%d\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Boxes: %d\n", result.Stats.TotalBins)

	// Output:
	// Fitted: 3/3
	// Boxes: 2
}

func ExamplePack_optimize() {
	boxTypes := []*model.Bin{
		model.NewBin("small", 30, 25, 20, 5),
		model.NewBin("large", 60, 50, 40, 25),
	}
	items := []*model.Item{
		model.NewItem("a", 25, 20, 15, 2),
		model.NewItem("b", 50, 40, 30, 10),
		model.NewItem("c", 20, 15, 10, 1),
	}

	result, err := gopackx.Pack(context.Background(), boxTypes, items,
		gopackx.Optimize(),
	)
	if err != nil {
		fmt.Println("error:", err)
		return
	}

	fmt.Printf("Fitted: %d/%d\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("Boxes: %d\n", result.Stats.TotalBins)

	// Output:
	// Fitted: 3/3
	// Boxes: 2
}
