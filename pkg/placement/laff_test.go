package placement

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestLAFFPlaceFirstItem(t *testing.T) {
	engine := NewLAFFEngine()
	bin := model.NewBin("box", 20, 20, 20, 100)
	item := model.NewItem("i1", 10, 10, 10, 5)

	if !engine.PlaceItem(bin, item) {
		t.Fatal("failed to place first item")
	}
	if len(engine.levels) != 1 {
		t.Errorf("expected 1 level, got %d", len(engine.levels))
	}
	if engine.levels[0].height != 10 {
		t.Errorf("level height = %f, want 10", engine.levels[0].height)
	}
}

func TestLAFFPlaceItemTooLarge(t *testing.T) {
	engine := NewLAFFEngine()
	bin := model.NewBin("box", 5, 5, 5, 100)
	item := model.NewItem("big", 10, 10, 10, 1)

	if engine.PlaceItem(bin, item) {
		t.Error("should not place item larger than bin")
	}
}

func TestLAFFMultipleLevels(t *testing.T) {
	engine := NewLAFFEngine()
	bin := model.NewBin("box", 10, 30, 10, 100)

	// Three items, each 10x10x10, should create 3 levels stacked vertically.
	for i := range 3 {
		item := model.NewItem("i", 10, 10, 10, 1)
		if !engine.PlaceItem(bin, item) {
			t.Fatalf("failed to place item %d", i)
		}
	}

	if len(bin.Items) != 3 {
		t.Errorf("bin.Items = %d, want 3", len(bin.Items))
	}
	if len(engine.levels) != 3 {
		t.Errorf("levels = %d, want 3", len(engine.levels))
	}
}

func TestLAFFNoIntersections(t *testing.T) {
	engine := NewLAFFEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	for range 10 {
		item := model.NewItem("i", 10, 10, 10, 1)
		if !engine.PlaceItem(bin, item) {
			break
		}
	}

	for i := range len(bin.Items) {
		for j := i + 1; j < len(bin.Items); j++ {
			if intersection.Intersect(bin.Items[i], bin.Items[j]) {
				t.Errorf("items %d and %d intersect", i, j)
			}
		}
	}
}

func TestLAFFWithinBounds(t *testing.T) {
	engine := NewLAFFEngine()
	bin := model.NewBin("box", 30, 30, 30, 1000)

	for range 15 {
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

func TestLAFFFastVariant(t *testing.T) {
	engine := NewLAFFEngine(LAFFFast())
	bin := model.NewBin("box", 30, 30, 30, 1000)

	placed := 0
	for range 10 {
		item := model.NewItem("i", 10, 10, 10, 1)
		if engine.PlaceItem(bin, item) {
			placed++
		}
	}

	if placed == 0 {
		t.Error("fast LAFF should place at least some items")
	}

	// Verify no intersections.
	for i := range len(bin.Items) {
		for j := i + 1; j < len(bin.Items); j++ {
			if intersection.Intersect(bin.Items[i], bin.Items[j]) {
				t.Errorf("items %d and %d intersect", i, j)
			}
		}
	}
}

func TestLAFFLargestAreaRotation(t *testing.T) {
	engine := NewLAFFEngine()
	// Tall narrow bin: 10 wide, 100 tall, 10 deep.
	bin := model.NewBin("tall", 10, 100, 10, 1000)

	// Item 5x20x8: best base area is 10x8=80 with rotation DWH (5→height).
	item := model.NewItem("i1", 5, 20, 8, 1)
	if !engine.PlaceItem(bin, item) {
		t.Fatal("failed to place item")
	}

	// The level height should be minimized for better bin utilization.
	// Rotation WHD: h=20, area=5*8=40
	// Rotation DWH: h=8, area=5*20=100 (but 20>10 width, doesn't fit)
	// Rotation WDH: h=8, area=5*20=100 (but 20>10 depth, doesn't fit)
	// Rotation HWD: h=5, area=20*8=160 (but 20>10 width, doesn't fit)
	// Rotation HDW: h=8, area=20*5=100 (but 20>10 width, doesn't fit)
	// Rotation DHW: h=20, area=8*5=40
	// Only WHD and DHW fit: both h=20. LAFF picks largest area → tie, either works.
	t.Logf("level height = %f, item rotation = %d", engine.levels[0].height, item.RotationType)
}

func TestLAFFMixedSizes(t *testing.T) {
	engine := NewLAFFEngine()
	bin := model.NewBin("box", 50, 50, 50, 1000)

	sizes := [][3]float64{
		{20, 10, 20}, // large flat
		{10, 10, 10}, // medium cube
		{5, 5, 5},    // small cube
		{15, 5, 10},  // medium flat
		{10, 15, 10}, // tall
	}

	placed := 0
	for _, s := range sizes {
		item := model.NewItem("i", s[0], s[1], s[2], 1)
		if engine.PlaceItem(bin, item) {
			placed++
		}
	}

	if placed != len(sizes) {
		t.Errorf("placed %d/%d items", placed, len(sizes))
	}

	t.Logf("levels=%d volume=%.1f%%", len(engine.levels), bin.VolumeUsedPct())
}
