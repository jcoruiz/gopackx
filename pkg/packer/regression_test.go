package packer_test

import (
	"context"
	"math"
	"reflect"
	"strconv"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/stability"
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

func layout(r *model.Result) [][]string {
	var out [][]string
	for _, b := range r.Bins {
		var ids []string
		for _, it := range b.Items {
			ids = append(ids, it.ID+"@"+strconv.FormatFloat(it.Position[0], 'g', -1, 64)+","+
				strconv.FormatFloat(it.Position[1], 'g', -1, 64)+","+strconv.FormatFloat(it.Position[2], 'g', -1, 64))
		}
		out = append(out, ids)
	}
	return out
}

// Pack used to fill the added bins in place: a second call appended the
// items again (6 items in a bin, stats saying 3) and the caller's items
// came back modified.
func TestPackIsRepeatableAndLeavesInputsUntouched(t *testing.T) {
	bin := model.NewBin("b", 30, 30, 30, 100)
	items := []*model.Item{
		model.NewItem("0", 10, 10, 10, 1),
		model.NewItem("1", 20, 10, 10, 1),
		model.NewItem("2", 10, 20, 10, 1),
	}
	p := packer.NewPacker(packer.WithPlacementEngine(placement.NewExtremePointEngine()))
	p.AddBin(bin)
	for _, it := range items {
		p.AddItem(it)
	}

	first, err := p.Pack(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	second, err := p.Pack(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(layout(first), layout(second)) {
		t.Errorf("second Pack differs:\n%v\n%v", layout(first), layout(second))
	}
	if n := len(second.Bins[0].Items); n != second.Stats.FittedItems || n != 3 {
		t.Errorf("bin holds %d items, stats say %d, want 3", n, second.Stats.FittedItems)
	}
	if len(bin.Items) != 0 || bin.TotalWeight() != 0 {
		t.Error("the added bin was modified")
	}
	for _, it := range items {
		if it.Placed || it.Position != [3]float64{} {
			t.Errorf("item %s was modified", it.ID)
		}
	}
}

// Items already placed in an added bin are kept, in every run.
func TestPackKeepsItemsAlreadyInABin(t *testing.T) {
	bin := model.NewBin("b", 20, 10, 10, 100)
	pre := model.NewItem("pre", 10, 10, 10, 1)
	pre.Position = [3]float64{0, 0, 0}
	bin.PlaceItem(pre)

	p := packer.NewPacker()
	p.AddBin(bin)
	p.AddItem(model.NewItem("new", 10, 10, 10, 1))
	for range 2 {
		r, err := p.Pack(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		got := r.Bins[0].Items
		if len(got) != 2 || got[0].ID != "pre" || got[1].Position[0] != 10 {
			t.Fatalf("bin holds %v, want pre at x=0 and new at x=10", layout(r))
		}
	}
	if len(bin.Items) != 1 {
		t.Error("the added bin was modified")
	}
}

// The load a bin tracks as items are placed matches stability.LoadOnTop
// computed from scratch, and every load limit holds, for every engine.
func TestTrackedLoadsMatchLoadsFromScratch(t *testing.T) {
	all := append(append([]engineSpec{}, engines...), func() []engineSpec {
		var out []engineSpec
		for _, e := range enginesWithStability {
			out = append(out, engineSpec(e))
		}
		return out
	}()...)
	for _, eng := range all {
		t.Run(eng.name, func(t *testing.T) {
			for seed := range 20 {
				p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
				p.AddBin(model.NewBin("a", 40, 40, 40, 0))
				p.AddBin(model.NewBin("b", 30, 50, 30, 0))
				for i := range 30 {
					v := seed*31 + i*17
					var opts []model.ItemOption
					if v%3 == 0 {
						opts = append(opts, model.ItemLoadBear(float64(5+v%20)))
					}
					w := float64(5 * (1 + v%4))
					p.AddItem(model.NewItem("i"+strconv.Itoa(i), w, float64(5*(1+v%3)), float64(5*(1+(v/3)%4)), float64(1+v%9), opts...))
				}
				res, err := p.Pack(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				for _, bin := range res.Bins {
					for i, it := range bin.Items {
						want := stability.LoadOnTop(it, bin.Items)
						if math.Abs(bin.Load(i)-want) > 1e-6 {
							t.Fatalf("seed %d: Load(%s) = %v, from scratch %v", seed, it.ID, bin.Load(i), want)
						}
						if it.LoadBear > 0 && want > it.LoadBear+1e-6 {
							t.Errorf("seed %d: %s carries %.2f kg, limit %.2f", seed, it.ID, want, it.LoadBear)
						}
					}
				}
			}
		})
	}
}
