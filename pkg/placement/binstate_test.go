package placement

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

type spot struct {
	bin int
	pos [3]float64
	rt  model.RotationType
}

// packAlternating packs 150 items into three bins, starting each item at a
// different bin so that engines keep switching. engineFor returns the
// engine to use for a bin on each call.
func packAlternating(engineFor func(bin int) Engine) []spot {
	bins := []*model.Bin{
		model.NewBin("a", 40, 30, 30, 1e9),
		model.NewBin("b", 50, 30, 30, 1e9),
		model.NewBin("c", 60, 40, 40, 1e9),
	}
	out := make([]spot, 0, 150)
	for i := range 150 {
		it := model.NewItem(fmt.Sprint("i", i), float64(5+i*7%17), float64(4+i*5%13), float64(6+i*11%19), 1)
		where := spot{bin: -1}
		for k := range bins {
			b := (i + k) % len(bins)
			if engineFor(b).PlaceItem(bins[b], it) {
				where = spot{b, it.Position, it.RotationType}
				break
			}
		}
		out = append(out, where)
	}
	return out
}

var engineFactories = map[string]func() Engine{
	"ExtremePoint":     func() Engine { return NewExtremePointEngine() },
	"ExtremePoint-S70": func() Engine { return NewExtremePointEngine(WithEPStability(0.7)) },
	"MaxRects":         func() Engine { return NewMaxRectsEngine() },
	"MaxRects-S70":     func() Engine { return NewMaxRectsEngine(WithMaxRectsStability(0.7)) },
	"LAFF":             func() Engine { return NewLAFFEngine() },
	"LAFFFast":         func() Engine { return NewLAFFEngine(LAFFFast()) },
	"Pivot":            func() Engine { return NewPivotEngine() },
}

func dedicated(newEngine func() Engine) func(int) Engine {
	perBin := map[int]Engine{}
	return func(b int) Engine {
		if perBin[b] == nil {
			perBin[b] = newEngine()
		}
		return perBin[b]
	}
}

func compareSpots(t *testing.T, got, want []spot, gotName, wantName string) {
	t.Helper()
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("item %d: %s put it at %+v, %s at %+v", i, gotName, got[i], wantName, want[i])
		}
	}
}

// One engine switching between bins must place items exactly like one
// engine per bin, otherwise results would depend on how a solver
// interleaves bins.
func TestSharedEngineMatchesEnginePerBin(t *testing.T) {
	for name, newEngine := range engineFactories {
		t.Run(name, func(t *testing.T) {
			shared := newEngine()
			got := packAlternating(func(int) Engine { return shared })
			compareSpots(t, got, packAlternating(dedicated(newEngine)), "shared engine", "engine per bin")
		})
	}
}

// Engines rebuild a bin they have not seen exactly as if they had packed
// it, so a fresh engine on every call gives the same result.
func TestRebuiltStateMatchesIncremental(t *testing.T) {
	for name, newEngine := range engineFactories {
		t.Run(name, func(t *testing.T) {
			got := packAlternating(func(int) Engine { return newEngine() })
			compareSpots(t, got, packAlternating(dedicated(newEngine)), "fresh engine per call", "engine per bin")
		})
	}
}

func TestBinStatesEvictsLeastRecentlyUsed(t *testing.T) {
	var c binStates[int]
	bins := make([]*model.Bin, maxBinStates+1)
	for i := range bins {
		bins[i] = model.NewBin(strconv.Itoa(i), 1, 1, 1, 1)
	}
	for i := range maxBinStates {
		c.save(bins[i], i, bins[i].Revision())
	}
	c.save(bins[0], 0, bins[0].Revision()) // bins[0] used again: bins[1] is now the oldest
	c.save(bins[maxBinStates], maxBinStates, bins[maxBinStates].Revision())
	if _, ok := c.load(bins[1]); ok {
		t.Error("least recently used bin should have been forgotten")
	}
	if v, ok := c.load(bins[0]); !ok || v != 0 {
		t.Error("recently used bin should still be remembered")
	}
	if len(c.m) != maxBinStates {
		t.Errorf("cache holds %d bins, want %d", len(c.m), maxBinStates)
	}
}

// An engine whose bin changed behind its back (items removed) must not keep
// using, or save, state that no longer matches the bin.
func TestEngineStateFollowsRemovedItems(t *testing.T) {
	for name, newEngine := range engineFactories {
		t.Run(name, func(t *testing.T) {
			cube := func() *model.Item { return model.NewItem("cube", 1, 1, 1, 1) }

			// Same bin: place, remove, place again.
			e := newEngine()
			a := model.NewBin("a", 1, 1, 1, 0)
			if !e.PlaceItem(a, cube()) {
				t.Fatal("first cube not placed")
			}
			a.RemoveLastItem()
			if !e.PlaceItem(a, cube()) {
				t.Error("the bin is empty again, but the engine still saw it full")
			}

			// Switching away after the removal and coming back.
			e = newEngine()
			a = model.NewBin("a", 1, 1, 1, 0)
			b := model.NewBin("b", 1, 1, 1, 0)
			e.PlaceItem(a, cube())
			a.RemoveLastItem()
			e.PlaceItem(b, cube())
			if !e.PlaceItem(a, cube()) {
				t.Error("after switching back, the engine restored state from before the removal")
			}
		})
	}
}

// After Reset an engine forgets every bin and places like a new engine.
func TestResetForgetsEveryBin(t *testing.T) {
	for name, newEngine := range engineFactories {
		t.Run(name, func(t *testing.T) {
			used := newEngine()
			first := model.NewBin("first", 20, 20, 20, 0)
			for i := range 5 {
				used.PlaceItem(first, model.NewItem(strconv.Itoa(i), 5, 5, 5, 1))
			}
			used.(Resetter).Reset()

			second, fresh := model.NewBin("second", 20, 20, 20, 0), model.NewBin("fresh", 20, 20, 20, 0)
			fe := newEngine()
			for i := range 5 {
				a, b := model.NewItem(strconv.Itoa(i), 6, 4, 5, 1), model.NewItem(strconv.Itoa(i), 6, 4, 5, 1)
				okA, okB := used.PlaceItem(second, a), fe.PlaceItem(fresh, b)
				if okA != okB || a.Position != b.Position {
					t.Fatalf("item %d: reset engine %v at %v, new engine %v at %v", i, okA, a.Position, okB, b.Position)
				}
			}
		})
	}
	var ep ExtremePointEngine
	ep.PlaceItem(model.NewBin("b", 5, 5, 5, 0), model.NewItem("x", 1, 1, 1, 1))
	ep.Reset()
	if ep.bin != nil || ep.points != nil || ep.saved.m != nil {
		t.Error("Reset should drop the extreme point engine's state")
	}
}

// A point that ends up strictly inside a placed item is removed, together
// with its key, so the position can be used again later.
func TestExtremePointsInsideAPlacedItemAreRemoved(t *testing.T) {
	e := NewExtremePointEngine()
	bin := model.NewBin("b", 20, 20, 20, 0)
	e.initBin(bin)
	inside := &ExtremePoint{Pos: [3]float64{5, 5, 5}, MaxSpace: [3]float64{1, 1, 1}}
	e.points = append(e.points, inside)
	e.keys[pointKey(inside.Pos)] = struct{}{}

	it := model.NewItem("block", 10, 10, 10, 1)
	bin.PlaceItem(it) // at the origin, covering the point
	e.items = bin.Items
	e.onItemPlaced(it)

	for _, p := range e.points {
		if p == inside {
			t.Fatal("the point inside the item was kept")
		}
	}
	if _, ok := e.keys[pointKey(inside.Pos)]; ok {
		t.Error("the removed point's key was kept")
	}
	if !e.isInsideAnyItem([3]float64{5, 5, 5}) || e.isInsideAnyItem([3]float64{15, 5, 5}) {
		t.Error("isInsideAnyItem should tell points inside the block from points outside")
	}
}

func TestCanPlaceRejectsNegativePositionsAndFragileItemsUnderOthers(t *testing.T) {
	bin := model.NewBin("b", 20, 20, 20, 0)
	shelf := model.NewItem("shelf", 10, 2, 10, 1)
	shelf.Position = [3]float64{0, 10, 0}
	bin.PlaceItem(shelf)

	neg := model.NewItem("neg", 2, 2, 2, 1)
	neg.Position = [3]float64{-1, 0, 0}
	if got := canPlaceDimBlocker(bin, neg, neg.Dimension(), false, 0); got != -2 {
		t.Errorf("negative position: got %d, want -2", got)
	}

	// A fragile box that would end up right under the floating shelf.
	glass := model.NewItem("glass", 4, 10, 4, 1, model.ItemFragile())
	glass.Position = [3]float64{0, 0, 0}
	if got := canPlaceDimBlocker(bin, glass, glass.Dimension(), false, 0); got != -2 {
		t.Errorf("fragile under the shelf: got %d, want -2", got)
	}
}
