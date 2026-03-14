package strategy_test

import (
	"fmt"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

func ExampleSortItems() {
	items := []*model.Item{
		model.NewItem("small", 5, 5, 5, 1),   // volume=125
		model.NewItem("large", 20, 20, 20, 8), // volume=8000
		model.NewItem("mid", 10, 10, 10, 3),   // volume=1000
	}

	strategy.SortItems(items, strategy.BestFitDecreasing)

	for _, it := range items {
		fmt.Printf("%s (vol=%.0f)\n", it.ID, it.Volume)
	}

	// Output:
	// large (vol=8000)
	// mid (vol=1000)
	// small (vol=125)
}

func ExampleSortBinsForItem() {
	bins := []*model.Bin{
		model.NewBin("big", 100, 100, 100, 500),
		model.NewBin("small", 30, 30, 30, 500),
		model.NewBin("medium", 50, 50, 50, 500),
	}

	item := model.NewItem("box", 10, 10, 10, 5)
	sorted := strategy.SortBinsForItem(bins, item, strategy.BestFit)

	for _, b := range sorted {
		fmt.Printf("%s (remaining=%.0f)\n", b.ID, strategy.RemainingVolume(b))
	}

	// Output:
	// small (remaining=27000)
	// medium (remaining=125000)
	// big (remaining=1000000)
}

func ExampleRemainingVolume() {
	bin := model.NewBin("bin-1", 100, 100, 100, 500)
	fmt.Printf("empty: %.0f\n", strategy.RemainingVolume(bin))

	item := model.NewItem("box", 50, 50, 50, 10)
	item.Position = [3]float64{0, 0, 0}
	bin.PlaceItem(item)
	fmt.Printf("after placing 50x50x50: %.0f\n", strategy.RemainingVolume(bin))

	// Output:
	// empty: 1000000
	// after placing 50x50x50: 875000
}
