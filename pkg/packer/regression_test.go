package packer_test

import (
	"context"
	"math"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
)

// weightOnTop returns the weight resting directly on top of item a, each
// item above counted by the share of its base that touches a.
func weightOnTop(a *model.Item, items []*model.Item) float64 {
	ad := a.Dimension()
	top := a.Position[1] + ad[1]
	total := 0.0
	for _, o := range items {
		if o == a || math.Abs(o.Position[1]-top) > 1e-6 {
			continue
		}
		od := o.Dimension()
		ow := overlapLen(a.Position[0], ad[0], o.Position[0], od[0])
		oz := overlapLen(a.Position[2], ad[2], o.Position[2], od[2])
		total += o.Weight * ow * oz / (od[0] * od[2])
	}
	return total
}

// A fragile item must not be slid into a gap under an item placed earlier.
// Found by fuzzing: "i0" overhangs "i1", then the fragile "i6" fitted under
// the overhang with its top touching i0's bottom.
func TestFragileItemNotSlidUnderOverhang(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			p.AddBin(model.NewBin("box", 30, 30, 30, 1000))
			p.AddItem(model.NewItem("i0", 10, 10, 20, 1))
			p.AddItem(model.NewItem("i1", 25, 20, 10, 1))
			p.AddItem(model.NewItem("i2", 30, 5, 20, 1))
			p.AddItem(model.NewItem("i4", 10, 15, 10, 1))
			p.AddItem(model.NewItem("i5", 5, 15, 25, 1))
			p.AddItem(model.NewItem("i6", 15, 5, 15, 1, model.ItemFragile()))

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			checkFragile(t, result.Bins)
		})
	}
}

// A load-limited item must not be slid under an overhang that puts more
// weight on it than it can hold. Found by fuzzing: "i0" (8 kg) overhangs
// "i1" with 80% support, and "i2" (holds 1 kg) fitted under the overhang,
// carrying 1.6 kg.
func TestLoadLimitedItemNotSlidUnderOverhang(t *testing.T) {
	for _, eng := range enginesWithStability {
		t.Run(eng.name, func(t *testing.T) {
			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			p.AddBin(model.NewBin("box", 30, 30, 30, 1000))
			p.AddItem(model.NewItem("i0", 25, 30, 10, 8))
			p.AddItem(model.NewItem("i1", 30, 20, 20, 1))
			p.AddItem(model.NewItem("i2", 30, 5, 20, 3, model.ItemLoadBear(1)))

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, bin := range result.Bins {
				for _, it := range bin.Items {
					if it.LoadBear > 0 {
						if w := weightOnTop(it, bin.Items); w > it.LoadBear+1e-6 {
							t.Errorf("%s holds %.2f kg, limit %.2f kg", it.ID, w, it.LoadBear)
						}
					}
				}
			}
		})
	}
}
