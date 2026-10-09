package stats

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestCompute(t *testing.T) {
	limited := model.NewBin("limited", 10, 10, 10, 10, model.BinCost(2))
	a := model.NewItem("a", 10, 10, 5, 4)
	limited.PlaceItem(a)
	free := model.NewBin("free", 10, 10, 10, 0, model.BinCost(3))
	b := model.NewItem("b", 10, 10, 10, 1000)
	free.PlaceItem(b)
	empty := model.NewBin("empty", 10, 10, 10, 10)
	c := model.NewItem("c", 1, 1, 1, 1)

	s := Compute([]*model.Bin{limited, free, empty}, []*model.Item{a, b, c}, []*model.Item{c})
	want := model.PackingStats{
		TotalBins:     2,
		TotalItems:    3,
		FittedItems:   2,
		UnfittedCount: 1,
		VolumeUsedPct: 75, // (50% + 100%) / 2
		WeightUsedPct: 40, // only the limited bin: 4 of 10 kg
		TotalCost:     5,
	}
	if s != want {
		t.Errorf("Compute() = %+v, want %+v", s, want)
	}
}
