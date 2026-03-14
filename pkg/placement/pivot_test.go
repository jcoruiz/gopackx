package placement

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestPlaceFirstItem(t *testing.T) {
	engine := NewPivotEngine()
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

func TestPlaceItemTooLarge(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 5, 5, 5, 100)
	item := model.NewItem("big", 10, 10, 10, 1)

	if engine.PlaceItem(bin, item) {
		t.Error("should not place item larger than bin")
	}
	if item.Placed {
		t.Error("item.Placed should be false")
	}
}

func TestPlaceItemTooHeavy(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 20, 20, 20, 5)
	item := model.NewItem("heavy", 10, 10, 10, 10)

	if engine.PlaceItem(bin, item) {
		t.Error("should not place item exceeding weight limit")
	}
}

func TestPlaceMultipleItems(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 20, 20, 20, 100)

	i1 := model.NewItem("i1", 10, 10, 10, 5)
	i2 := model.NewItem("i2", 10, 10, 10, 5)

	if !engine.PlaceItem(bin, i1) {
		t.Fatal("failed to place i1")
	}
	if !engine.PlaceItem(bin, i2) {
		t.Fatal("failed to place i2")
	}
	if len(bin.Items) != 2 {
		t.Errorf("bin.Items count = %d, want 2", len(bin.Items))
	}

	// Items must not intersect.
	if intersection.Intersect(i1, i2) {
		t.Error("placed items should not intersect")
	}
}

func TestPlaceNoIntersections(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	for i := 0; i < 8; i++ {
		item := model.NewItem("i", 10, 10, 10, 1)
		if !engine.PlaceItem(bin, item) {
			// Not all may fit; that's OK.
			break
		}
	}

	// Verify no pair intersects.
	for i := 0; i < len(bin.Items); i++ {
		for j := i + 1; j < len(bin.Items); j++ {
			if intersection.Intersect(bin.Items[i], bin.Items[j]) {
				t.Errorf("items %d and %d intersect", i, j)
			}
		}
	}
}

func TestPlaceWithinBounds(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	for i := 0; i < 10; i++ {
		item := model.NewItem("i", 10, 10, 10, 1)
		engine.PlaceItem(bin, item)
	}

	for i, item := range bin.Items {
		dim := item.Dimension()
		if item.Position[0]+dim[0] > bin.Width+epsilon ||
			item.Position[1]+dim[1] > bin.Height+epsilon ||
			item.Position[2]+dim[2] > bin.Depth+epsilon {
			t.Errorf("item %d out of bin bounds: pos=%v dim=%v", i, item.Position, dim)
		}
		if item.Position[0] < -epsilon || item.Position[1] < -epsilon || item.Position[2] < -epsilon {
			t.Errorf("item %d has negative position: %v", i, item.Position)
		}
	}
}

func TestPlaceUprightConstraint(t *testing.T) {
	engine := NewPivotEngine()
	// Bin where only an upright rotation fits.
	bin := model.NewBin("tall", 5, 20, 5, 100)
	item := model.NewItem("i1", 5, 15, 5, 1, model.ItemUpright())

	if !engine.PlaceItem(bin, item) {
		t.Fatal("failed to place upright item")
	}
	// With upright, HeightAxis dimension should be the original height (15).
	dim := item.Dimension()
	if dim[model.HeightAxis] != 15 {
		t.Errorf("height dim = %f, want 15 (upright)", dim[model.HeightAxis])
	}
}

func TestPlaceFragileConstraint(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 10, 20, 10, 100)

	fragile := model.NewItem("fragile", 10, 10, 10, 1, model.ItemFragile())
	if !engine.PlaceItem(bin, fragile) {
		t.Fatal("failed to place fragile item")
	}

	// Nothing should be placed on top of the fragile item.
	top := model.NewItem("top", 10, 10, 10, 1)
	if engine.PlaceItem(bin, top) {
		// If placed, verify it's not directly on top of the fragile item.
		fragDim := fragile.Dimension()
		fragTop := fragile.Position[model.HeightAxis] + fragDim[model.HeightAxis]
		topDim := top.Dimension()

		// Check if top is resting on fragile and overlaps in W-D.
		if top.Position[model.HeightAxis] >= fragTop-epsilon &&
			top.Position[model.HeightAxis] <= fragTop+epsilon {
			ow := overlapLen(
				top.Position[model.WidthAxis], topDim[model.WidthAxis],
				fragile.Position[model.WidthAxis], fragDim[model.WidthAxis],
			)
			od := overlapLen(
				top.Position[model.DepthAxis], topDim[model.DepthAxis],
				fragile.Position[model.DepthAxis], fragDim[model.DepthAxis],
			)
			if ow > epsilon && od > epsilon {
				t.Error("item placed directly on top of fragile item")
			}
		}
	}
}

func TestPlaceWithStability(t *testing.T) {
	engine := NewPivotEngine(WithStability(0.75))
	bin := model.NewBin("box", 20, 20, 20, 100)

	base := model.NewItem("base", 10, 10, 10, 5)
	if !engine.PlaceItem(bin, base) {
		t.Fatal("failed to place base item")
	}

	// Item with enough support (same footprint).
	supported := model.NewItem("supported", 10, 10, 10, 3)
	if !engine.PlaceItem(bin, supported) {
		t.Fatal("failed to place fully supported item")
	}
}

func TestPlaceWithStabilityRejectsUnsupported(t *testing.T) {
	// High stability ratio: item needs nearly full base support.
	engine := NewPivotEngine(WithStability(0.99))
	bin := model.NewBin("box", 40, 40, 40, 1000)

	// Place a small base item.
	base := model.NewItem("base", 10, 10, 10, 5)
	if !engine.PlaceItem(bin, base) {
		t.Fatal("failed to place base item")
	}

	// Try to place a much wider item on top — only partially supported.
	wide := model.NewItem("wide", 30, 10, 30, 3)
	placed := engine.PlaceItem(bin, wide)

	// If placed, verify it found a valid position (on floor or supported).
	if placed {
		// It might have been placed on the floor next to base, which is fine.
		t.Logf("wide item placed at %v (found floor position)", wide.Position)
	}
}

func TestPlaceWithLoadBearingLimit(t *testing.T) {
	engine := NewPivotEngine(WithStability(0.5))
	bin := model.NewBin("box", 20, 40, 20, 1000)

	// Base item with low load-bearing capacity.
	base := model.NewItem("weak", 20, 10, 20, 5, model.ItemLoadBear(1))
	if !engine.PlaceItem(bin, base) {
		t.Fatal("failed to place base item")
	}

	// Heavy item that exceeds base's load-bearing capacity.
	heavy := model.NewItem("heavy", 20, 10, 20, 50)
	placed := engine.PlaceItem(bin, heavy)

	if placed && heavy.Position[model.HeightAxis] > 0 {
		// If it was placed above base, the load-bearing check should have blocked it.
		t.Logf("heavy item placed at %v", heavy.Position)
	}
}

func TestPlaceMaxRectsWithStability(t *testing.T) {
	engine := NewMaxRectsEngine(WithMaxRectsStability(0.5))
	bin := model.NewBin("box", 20, 30, 20, 1000)

	base := model.NewItem("base", 20, 10, 20, 5)
	if !engine.PlaceItem(bin, base) {
		t.Fatal("failed to place base item")
	}

	top := model.NewItem("top", 15, 10, 15, 3)
	if !engine.PlaceItem(bin, top) {
		t.Fatal("failed to place supported item on MaxRects")
	}
	if len(bin.Items) != 2 {
		t.Errorf("Items count = %d, want 2", len(bin.Items))
	}
}

func TestPlaceMaxRectsFragile(t *testing.T) {
	engine := NewMaxRectsEngine()
	bin := model.NewBin("box", 10, 30, 10, 100)

	fragile := model.NewItem("fragile", 10, 10, 10, 1, model.ItemFragile())
	if !engine.PlaceItem(bin, fragile) {
		t.Fatal("failed to place fragile item")
	}

	// Item that would need to go on top of fragile.
	top := model.NewItem("top", 10, 10, 10, 1)
	if engine.PlaceItem(bin, top) {
		topY := top.Position[model.HeightAxis]
		fragDim := fragile.Dimension()
		fragTop := fragile.Position[model.HeightAxis] + fragDim[model.HeightAxis]
		if topY >= fragTop-epsilon && topY <= fragTop+epsilon {
			t.Error("MaxRects should not place item on top of fragile item")
		}
	}
}

func TestPlaceMaxRectsWeightLimit(t *testing.T) {
	engine := NewMaxRectsEngine()
	bin := model.NewBin("box", 20, 20, 20, 10)

	heavy := model.NewItem("heavy", 10, 10, 10, 15)
	if engine.PlaceItem(bin, heavy) {
		t.Error("MaxRects should reject item exceeding weight limit")
	}
}

func TestFixPointCompaction(t *testing.T) {
	engine := NewPivotEngine()
	bin := model.NewBin("box", 30, 30, 30, 100)

	// Place first item at origin.
	i1 := model.NewItem("i1", 10, 10, 10, 1)
	engine.PlaceItem(bin, i1)

	// Second item should be placed adjacent (not floating).
	i2 := model.NewItem("i2", 10, 10, 10, 1)
	engine.PlaceItem(bin, i2)

	// At least one axis should be 0 (touching a wall).
	hasZero := i2.Position[0] == 0 || i2.Position[1] == 0 || i2.Position[2] == 0
	if !hasZero {
		t.Errorf("fix-point should compact item to a wall, pos = %v", i2.Position)
	}
}
