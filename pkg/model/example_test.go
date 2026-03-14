package model_test

import (
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
)

func ExampleNewItem() {
	// Create a basic item with dimensions 10x20x15 and weight 5 kg.
	basic := model.NewItem("box-1", 10, 20, 15, 5)
	fmt.Printf("ID=%s Volume=%.0f Rotations=%d\n",
		basic.ID, basic.Volume, len(basic.AllowedRotations))

	// Create an upright fragile item with priority.
	fragile := model.NewItem("glass-1", 8, 12, 8, 2,
		model.ItemUpright(),
		model.ItemFragile(),
		model.ItemPriority(1),
	)
	fmt.Printf("ID=%s Upright=%v Fragile=%v Priority=%d Rotations=%d\n",
		fragile.ID, len(fragile.AllowedRotations) == 2, fragile.Fragile,
		fragile.Priority, len(fragile.AllowedRotations))

	// Create an item with load-bearing capacity and group.
	sturdy := model.NewItem("crate-1", 20, 10, 20, 15,
		model.ItemLoadBear(50),
		model.ItemGroup("electronics"),
	)
	fmt.Printf("ID=%s LoadBear=%.0f Group=%s\n",
		sturdy.ID, sturdy.LoadBear, sturdy.Group)

	// Output:
	// ID=box-1 Volume=3000 Rotations=6
	// ID=glass-1 Upright=true Fragile=true Priority=1 Rotations=2
	// ID=crate-1 LoadBear=50 Group=electronics
}

func ExampleNewBin() {
	bin := model.NewBin("container-1", 100, 100, 100, 500)
	fmt.Printf("ID=%s Volume=%.0f MaxWeight=%.0f\n",
		bin.ID, bin.Volume, bin.MaxWeight)
	fmt.Printf("RemainingWeight=%.0f UsedVolume=%.0f VolumeUsedPct=%.1f%%\n",
		bin.RemainingWeight(), bin.UsedVolume(), bin.VolumeUsedPct())

	// Output:
	// ID=container-1 Volume=1000000 MaxWeight=500
	// RemainingWeight=500 UsedVolume=0 VolumeUsedPct=0.0%
}

func ExampleItem_Dimension() {
	item := model.NewItem("box", 10, 20, 30, 5)

	// Default rotation (WHD): dimensions are unchanged.
	item.RotationType = model.RotationWHD
	fmt.Printf("WHD: %v\n", item.Dimension())

	// HWD rotation: height and width are swapped.
	item.RotationType = model.RotationHWD
	fmt.Printf("HWD: %v\n", item.Dimension())

	// DHW rotation: depth moves to width position.
	item.RotationType = model.RotationDHW
	fmt.Printf("DHW: %v\n", item.Dimension())

	// Output:
	// WHD: [10 20 30]
	// HWD: [20 10 30]
	// DHW: [30 20 10]
}
