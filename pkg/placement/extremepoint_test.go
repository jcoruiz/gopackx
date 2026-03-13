package placement

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestEPPlaceFirstItem(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 20, 20, 20, 100)
	item := model.NewItem("i1", 10, 10, 10, 5)

	if !engine.PlaceItem(bin, item) {
		t.Fatal("failed to place first item")
	}
	if !item.Placed {
		t.Error("item.Placed should be true")
	}
	if len(bin.Items) != 1 {
		t.Errorf("bin.Items count = %d, want 1", len(bin.Items))
	}
}

func TestEPPlaceItemTooLarge(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 5, 5, 5, 100)
	item := model.NewItem("big", 10, 10, 10, 1)

	if engine.PlaceItem(bin, item) {
		t.Error("should not place item larger than bin")
	}
}

func TestEPPlaceMultipleItems(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 20, 20, 20, 100)

	i1 := model.NewItem("i1", 10, 10, 10, 5)
	i2 := model.NewItem("i2", 10, 10, 10, 5)

	if !engine.PlaceItem(bin, i1) {
		t.Fatal("failed to place i1")
	}
	if !engine.PlaceItem(bin, i2) {
		t.Fatal("failed to place i2")
	}

	if intersection.Intersect(i1, i2) {
		t.Error("placed items should not intersect")
	}
}

func TestEPNoIntersections(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	for i := 0; i < 10; i++ {
		item := model.NewItem("i", 10, 10, 10, 1)
		if !engine.PlaceItem(bin, item) {
			break
		}
	}

	for i := 0; i < len(bin.Items); i++ {
		for j := i + 1; j < len(bin.Items); j++ {
			if intersection.Intersect(bin.Items[i], bin.Items[j]) {
				t.Errorf("items %d and %d intersect", i, j)
			}
		}
	}
}

func TestEPWithinBounds(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	for i := 0; i < 15; i++ {
		item := model.NewItem("i", 10, 10, 10, 1)
		engine.PlaceItem(bin, item)
	}

	for i, item := range bin.Items {
		dim := item.Dimension()
		if item.Position[0]+dim[0] > bin.Width+epsilon ||
			item.Position[1]+dim[1] > bin.Height+epsilon ||
			item.Position[2]+dim[2] > bin.Depth+epsilon {
			t.Errorf("item %d out of bounds: pos=%v dim=%v", i, item.Position, dim)
		}
	}
}

func TestEPPointGeneration(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	// After placing first item, should have new extreme points.
	item := model.NewItem("i1", 10, 10, 10, 1)
	engine.PlaceItem(bin, item)

	if len(engine.points) < 3 {
		t.Errorf("expected at least 3 extreme points after first placement, got %d", len(engine.points))
	}
}

func TestEPMaxSpaceFilter(t *testing.T) {
	engine := NewExtremePointEngine()
	bin := model.NewBin("box", 20, 10, 20, 1000)

	i1 := model.NewItem("i1", 10, 10, 20, 1)
	engine.PlaceItem(bin, i1)

	// All remaining points should have maxSpace consistent with the bin and item.
	for _, ep := range engine.points {
		if ep.MaxSpace[0] <= 0 || ep.MaxSpace[1] <= 0 || ep.MaxSpace[2] <= 0 {
			t.Errorf("point %v has zero maxSpace: %v", ep.Pos, ep.MaxSpace)
		}
	}
}

func TestEPBetterThanPivot(t *testing.T) {
	// EP should fit at least as many items as Pivot for identical inputs.
	items := func() []*model.Item {
		out := make([]*model.Item, 20)
		for i := range out {
			w := float64(5 + i%10)
			h := float64(3 + (i*3)%8)
			d := float64(4 + (i*7)%9)
			out[i] = model.NewItem("i", w, h, d, 1)
		}
		return out
	}

	pivotBin := model.NewBin("box", 50, 50, 50, 1000)
	pivot := NewPivotEngine()
	pivotItems := items()
	for _, item := range pivotItems {
		pivot.PlaceItem(pivotBin, item)
	}
	pivotFitted := len(pivotBin.Items)

	epBin := model.NewBin("box", 50, 50, 50, 1000)
	ep := NewExtremePointEngine()
	epItems := items()
	for _, item := range epItems {
		ep.PlaceItem(epBin, item)
	}
	epFitted := len(epBin.Items)

	t.Logf("pivot fitted=%d (%.1f%%), ep fitted=%d (%.1f%%)",
		pivotFitted, pivotBin.VolumeUsedPct(),
		epFitted, epBin.VolumeUsedPct())

	if epFitted < pivotFitted {
		t.Errorf("EP fitted %d < Pivot fitted %d", epFitted, pivotFitted)
	}
}
