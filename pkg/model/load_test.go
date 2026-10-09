package model

import (
	"math"
	"testing"
)

func at(it *Item, x, y, z float64) *Item {
	it.Position = [3]float64{x, y, z}
	return it
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// The load on an item counts everything stacked on it, not only what
// touches it.
func TestLoadCountsStackedWeight(t *testing.T) {
	b := NewBin("b", 10, 30, 10, 0)
	crate := at(NewItem("crate", 10, 10, 10, 5, ItemLoadBear(30)), 0, 0, 0)
	b.PlaceItem(crate)
	b.PlaceItem(at(NewItem("a", 10, 10, 10, 15), 0, 10, 0))
	if got := b.Load(0); !near(got, 15) {
		t.Fatalf("Load(crate) = %v, want 15", got)
	}

	heavy := NewItem("b", 10, 10, 10, 20)
	if b.FitsLoadLimits(heavy, [3]float64{0, 20, 0}, [3]float64{10, 10, 10}) {
		t.Error("20 kg on top of the 15 kg item would put 35 kg on a crate that holds 30")
	}
	light := NewItem("c", 10, 10, 10, 10)
	if !b.FitsLoadLimits(light, [3]float64{0, 20, 0}, [3]float64{10, 10, 10}) {
		t.Error("10 kg more makes 25 kg on the crate, within its 30 kg limit")
	}
	b.PlaceItem(at(light, 0, 20, 0))
	if got := b.Load(0); !near(got, 25) {
		t.Errorf("Load(crate) = %v, want 25", got)
	}
	if got := b.Load(1); !near(got, 10) {
		t.Errorf("Load(a) = %v, want 10", got)
	}
}

// An overhanging item's whole weight goes to what supports it.
func TestOverhangingItemPassesAllItsWeight(t *testing.T) {
	b := NewBin("b", 20, 20, 10, 0)
	b.PlaceItem(at(NewItem("x", 6, 10, 10, 1), 0, 0, 0))
	b.PlaceItem(at(NewItem("y", 10, 5, 10, 8), 0, 10, 0)) // 60% of its base on x
	if got := b.Load(0); !near(got, 8) {
		t.Errorf("Load(x) = %v, want 8: the overhang does not make weight vanish", got)
	}
}

// An item slid under an overhang takes its share of the weight above, and
// removing it gives that share back.
func TestSlidUnderItemTakesItsShare(t *testing.T) {
	b := NewBin("b", 20, 20, 10, 0)
	b.PlaceItem(at(NewItem("x", 6, 10, 10, 1), 0, 0, 0))
	b.PlaceItem(at(NewItem("y", 10, 5, 10, 8), 0, 10, 0))

	c := NewItem("c", 4, 10, 10, 1, ItemLoadBear(3))
	pos, dim := [3]float64{6, 0, 0}, [3]float64{4, 10, 10}
	if b.FitsLoadLimits(c, pos, dim) {
		t.Error("c would carry 40% of 8 kg = 3.2 kg, over its 3 kg limit")
	}
	c.LoadBear = 4
	if !b.FitsLoadLimits(c, pos, dim) {
		t.Fatal("c holds 4 kg and would carry 3.2 kg")
	}
	b.PlaceItem(at(c, 6, 0, 0))
	if !near(b.Load(0), 4.8) || !near(b.Load(2), 3.2) {
		t.Errorf("Load(x) = %v, Load(c) = %v, want 4.8 and 3.2", b.Load(0), b.Load(2))
	}

	b.RemoveLastItem()
	if !near(b.Load(0), 8) {
		t.Errorf("after removing c, Load(x) = %v, want 8", b.Load(0))
	}
}

// A load-limited item deep in a stack is protected from items placed at the
// top.
func TestLoadLimitHoldsThroughTheStack(t *testing.T) {
	b := NewBin("b", 10, 40, 10, 0)
	b.PlaceItem(at(NewItem("weak", 10, 10, 10, 1, ItemLoadBear(12)), 0, 0, 0))
	b.PlaceItem(at(NewItem("mid", 10, 10, 10, 5), 0, 10, 0))
	b.PlaceItem(at(NewItem("top", 10, 10, 10, 5), 0, 20, 0))
	if b.FitsLoadLimits(NewItem("more", 10, 10, 10, 5), [3]float64{0, 30, 0}, [3]float64{10, 10, 10}) {
		t.Error("15 kg would rest on an item that holds 12")
	}
}

// Loads computed when tracking starts late match loads tracked from the
// first item.
func TestLateLoadTrackingMatchesTrackingFromTheStart(t *testing.T) {
	place := func(b *Bin) {
		b.PlaceItem(at(NewItem("x", 6, 10, 10, 1), 0, 0, 0))
		b.PlaceItem(at(NewItem("y", 10, 5, 10, 8), 0, 10, 0))
		b.PlaceItem(at(NewItem("z", 10, 5, 10, 3), 0, 15, 0))
		b.PlaceItem(at(NewItem("c", 4, 10, 10, 1), 6, 0, 0)) // slid under y
		b.PlaceItem(at(NewItem("w", 5, 5, 10, 2), 0, 20, 0))
	}
	late := NewBin("late", 20, 30, 10, 0)
	place(late)
	if late.tracked {
		t.Fatal("a bin without load limits should not track loads while packing")
	}
	early := NewBin("early", 20, 30, 10, 0)
	early.tracked = true
	place(early)
	for i := range late.Items {
		if !near(late.Load(i), early.Load(i)) {
			t.Errorf("item %s: late tracking %v, from the start %v", late.Items[i].ID, late.Load(i), early.Load(i))
		}
	}
	if !near(late.Load(0), 0.6*(8+3+2)) {
		t.Errorf("Load(x) = %v, want 60%% of the 13 kg above", late.Load(0))
	}
}
