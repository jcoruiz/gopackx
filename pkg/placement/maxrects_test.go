package placement

import (
	"fmt"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

// Placing an item that overlaps several free spaces must keep the
// sub-spaces of every split, not only those of the last one. Before the fix
// a pallet that holds all these items used to leave most of them out.
func TestMaxRectsKeepsSpacesOfEverySplit(t *testing.T) {
	bin := model.NewBin("pallet", 120, 160, 100, 1e9)
	e := NewMaxRectsEngine()
	placed := 0
	for i := range 400 {
		it := model.NewItem(fmt.Sprint("i", i), float64(5+i*7%23), float64(4+i*5%19), float64(6+i*11%25), 1)
		if e.PlaceItem(bin, it) {
			placed++
		}
	}
	if placed < 390 {
		t.Errorf("placed %d of 400 items, want at least 390", placed)
	}
}

func TestKeepMaximal(t *testing.T) {
	big := freeSpace{0, 0, 0, 10, 10, 10}
	inside := freeSpace{1, 1, 1, 2, 2, 2}
	other := freeSpace{20, 0, 0, 5, 5, 5}

	got := keepMaximal([]freeSpace{big}, []freeSpace{inside, other, other})
	if len(got) != 1 || got[0] != other {
		t.Errorf("keepMaximal = %v, want only one copy of %v", got, other)
	}

	// A split space inside another split space is dropped too.
	got = keepMaximal(nil, []freeSpace{inside, big})
	if len(got) != 1 || got[0] != big {
		t.Errorf("keepMaximal = %v, want [%v]", got, big)
	}
}
