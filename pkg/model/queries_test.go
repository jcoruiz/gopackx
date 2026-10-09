package model

import (
	"errors"
	"testing"
)

func TestCollides(t *testing.T) {
	b := NewBin("b", 10, 10, 10, 0)
	b.PlaceItem(at(NewItem("a", 4, 4, 4, 1), 0, 0, 0))
	b.PlaceItem(at(NewItem("c", 4, 4, 4, 1), 5, 0, 0))

	if got := b.Collides([3]float64{3, 0, 0}, [3]float64{3, 3, 3}); got != 0 {
		t.Errorf("box overlapping a: Collides = %d, want 0", got)
	}
	if got := b.Collides([3]float64{6, 2, 1}, [3]float64{1, 1, 1}); got != 1 {
		t.Errorf("box inside c: Collides = %d, want 1", got)
	}
	// Touching faces are not an overlap.
	if got := b.Collides([3]float64{4, 0, 0}, [3]float64{1, 4, 4}); got != -1 {
		t.Errorf("box between a and c: Collides = %d, want -1", got)
	}
}

func TestSlideToOrigin(t *testing.T) {
	b := NewBin("b", 10, 10, 10, 0)
	b.PlaceItem(at(NewItem("floor", 10, 2, 10, 1), 0, 0, 0))
	b.PlaceItem(at(NewItem("wall", 3, 5, 10, 1), 0, 2, 0))

	// Drops onto the floor item, then slides left to the wall item and
	// back to the bin's front wall.
	got := b.SlideToOrigin([3]float64{6, 8, 4}, [3]float64{2, 2, 2})
	if want := [3]float64{3, 2, 0}; got != want {
		t.Errorf("SlideToOrigin = %v, want %v", got, want)
	}
}

func TestRestsOnFragileAndHasItemOnTop(t *testing.T) {
	b := NewBin("b", 10, 10, 10, 0)
	b.PlaceItem(at(NewItem("plain", 4, 4, 4, 1), 0, 0, 0))
	b.PlaceItem(at(NewItem("glass", 4, 4, 4, 1, ItemFragile()), 5, 0, 0))

	if got := b.RestsOnFragile([3]float64{5, 4, 0}, [3]float64{2, 2, 2}); got != 1 {
		t.Errorf("box on the glass: RestsOnFragile = %d, want 1", got)
	}
	if got := b.RestsOnFragile([3]float64{0, 4, 0}, [3]float64{2, 2, 2}); got != -1 {
		t.Errorf("box on the plain item: RestsOnFragile = %d, want -1", got)
	}
	if got := b.RestsOnFragile([3]float64{5, 5, 0}, [3]float64{2, 2, 2}); got != -1 {
		t.Errorf("box floating above the glass: RestsOnFragile = %d, want -1", got)
	}

	b.PlaceItem(at(NewItem("lid", 4, 1, 4, 1), 0, 4, 0))
	if !b.HasItemOnTop([3]float64{0, 0, 0}, [3]float64{4, 4, 4}) {
		t.Error("the lid rests on the plain item")
	}
	if b.HasItemOnTop([3]float64{5, 0, 0}, [3]float64{4, 4, 4}) {
		t.Error("nothing rests on the glass")
	}
}

func TestFitsLoadLimitsWithoutLimits(t *testing.T) {
	b := NewBin("b", 10, 10, 10, 0)
	if !b.FitsLoadLimits(NewItem("a", 1, 1, 1, 1), [3]float64{}, [3]float64{1, 1, 1}) {
		t.Error("an empty bin has no load limits to break")
	}
	b.PlaceItem(at(NewItem("x", 2, 2, 2, 1), 0, 0, 0))
	if !b.FitsLoadLimits(NewItem("a", 1, 1, 1, 1000), [3]float64{0, 2, 0}, [3]float64{1, 1, 1}) {
		t.Error("no item has a limit")
	}
	if b.tracked {
		t.Error("checking a bin without load limits should not start tracking loads")
	}
}

// Load reaches an item through two paths (a diamond), from supports at
// different heights, and an item beside the box puts nothing on it.
func TestLoadThroughSeveralPaths(t *testing.T) {
	b := NewBin("b", 20, 30, 10, 0)
	b.PlaceItem(at(NewItem("base", 10, 10, 10, 1, ItemLoadBear(100)), 0, 0, 0))
	b.PlaceItem(at(NewItem("tall", 4, 15, 10, 1), 10, 0, 0))   // on the floor
	b.PlaceItem(at(NewItem("left", 5, 5, 10, 1), 0, 10, 0))    // on base
	b.PlaceItem(at(NewItem("right", 5, 5, 10, 1), 5, 10, 0))   // on base
	b.PlaceItem(at(NewItem("top", 14, 5, 10, 14), 0, 15, 0))   // on left, right and tall
	b.PlaceItem(at(NewItem("beside", 6, 5, 10, 1), 14, 15, 0)) // on nothing it touches
	// top's 14 kg: 5/14 on each of left and right, 4/14 on tall. base gets
	// left's and right's own 1 kg plus their shares of top.
	if got, want := b.Load(0), 2+14*10.0/14; !near(got, want) {
		t.Errorf("Load(base) = %v, want %v", got, want)
	}
	if got, want := b.Load(1), 14*4.0/14; !near(got, want) {
		t.Errorf("Load(tall) = %v, want %v", got, want)
	}
	// A box slid under "beside" carries it, while "top", whose bottom is at
	// the same height but beside the box, puts nothing on it.
	b.PlaceItem(at(NewItem("under", 6, 5, 10, 1), 14, 10, 0))
	if got := b.Load(6); !near(got, 1) {
		t.Errorf("Load(under) = %v, want 1", got)
	}
	if !errors.Is(Validate([]*Bin{b}, nil), nil) {
		t.Error("a packed bin should stay valid")
	}
}
