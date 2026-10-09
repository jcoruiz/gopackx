package gopackx_test

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/solver"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// These tests check every result against the physical rules with code that
// does not use the library's own checks. Sizes snap to multiples of 5, as
// real box catalogs do: with continuous random sizes faces almost never
// touch, which hid fragile and load-limit bugs from earlier property tests.

type spec struct {
	w, h, d, weight  float64
	upright, fragile bool
	loadBear         float64
}

type scenario struct {
	bins  []*model.Bin
	items []*model.Item
	specs map[string]spec
}

func randomScenario(seed uint64) scenario {
	r := rand.New(rand.NewPCG(seed, 7))
	size := func(lo, hi int) float64 {
		v := lo + r.IntN(hi-lo+1)
		if r.IntN(2) == 0 {
			v = max(5, v/5*5)
		}
		return float64(v)
	}
	sc := scenario{specs: map[string]spec{}}
	for i := range 1 + r.IntN(3) {
		maxWeight := float64(30 + r.IntN(200))
		if r.IntN(4) == 0 {
			maxWeight = 0 // no limit
		}
		sc.bins = append(sc.bins, model.NewBin(fmt.Sprint("T", i), size(20, 60), size(20, 60), size(20, 60), maxWeight))
	}
	for i := range 3 + r.IntN(25) {
		s := spec{w: size(3, 30), h: size(3, 30), d: size(3, 30), weight: float64(1+r.IntN(40)) / 2}
		var opts []model.ItemOption
		if r.IntN(100) < 15 {
			s.fragile = true
			opts = append(opts, model.ItemFragile())
		}
		if r.IntN(100) < 20 {
			s.upright = true
			opts = append(opts, model.ItemUpright())
		}
		if r.IntN(100) < 30 {
			s.loadBear = float64(1 + r.IntN(25))
			opts = append(opts, model.ItemLoadBear(s.loadBear))
		}
		id := fmt.Sprint("i", i)
		sc.items = append(sc.items, model.NewItem(id, s.w, s.h, s.d, s.weight, opts...))
		sc.specs[id] = s
	}
	return sc
}

const tol = 1e-6

func overlap(a0, a1, b0, b1 float64) float64 { return math.Max(0, math.Min(a1, b1)-math.Max(a0, b0)) }

type box struct{ lo, hi [3]float64 }

func boxOf(it *model.Item) box {
	d := it.Dimension()
	p := it.Position
	return box{p, [3]float64{p[0] + d[0], p[1] + d[1], p[2] + d[2]}}
}

// contact is the area of a's base resting on b's top face.
func contact(a, b box) float64 {
	if math.Abs(a.lo[1]-b.hi[1]) > tol {
		return 0
	}
	return overlap(a.lo[0], a.hi[0], b.lo[0], b.hi[0]) * overlap(a.lo[2], a.hi[2], b.lo[2], b.hi[2])
}

// loads returns, per item, the weight of everything stacked on it: each
// item passes its weight plus its load down, split by contact area.
func loads(items []*model.Item) []float64 {
	n := len(items)
	boxes := make([]box, n)
	order := make([]int, n)
	for i, it := range items {
		boxes[i] = boxOf(it)
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool { return boxes[order[a]].lo[1] > boxes[order[b]].lo[1] })
	carried := make([]float64, n)
	load := make([]float64, n)
	for _, i := range order {
		carried[i] += items[i].Weight
		total := 0.0
		for j := range items {
			if j != i {
				total += contact(boxes[i], boxes[j])
			}
		}
		for j := range items {
			if c := contact(boxes[i], boxes[j]); j != i && c > tol {
				share := carried[i] * c / total
				load[j] += share
				carried[j] += share
			}
		}
	}
	return load
}

// physicalErrors lists every rule a result breaks.
func physicalErrors(sc scenario, res *model.Result, stability float64) []string {
	var errs []string
	fail := func(f string, a ...any) { errs = append(errs, fmt.Sprintf(f, a...)) }
	seen := map[string]int{}
	for _, b := range res.Bins {
		for _, it := range b.Items {
			seen[it.ID]++
		}
	}
	for _, it := range res.UnfittedItems {
		seen[it.ID]++
	}
	for _, it := range sc.items {
		if seen[it.ID] != 1 {
			fail("%s returned %d times", it.ID, seen[it.ID])
		}
	}
	for _, b := range res.Bins {
		weight := 0.0
		ld := loads(b.Items)
		for i, it := range b.Items {
			s, a := sc.specs[it.ID], boxOf(it)
			weight += it.Weight
			for k := range 3 {
				if a.lo[k] < -tol || a.hi[k] > [3]float64{b.Width, b.Height, b.Depth}[k]+tol {
					fail("%s sticks out of %s", it.ID, b.ID)
				}
			}
			for j := i + 1; j < len(b.Items); j++ {
				c := boxOf(b.Items[j])
				if overlap(a.lo[0], a.hi[0], c.lo[0], c.hi[0]) > tol && overlap(a.lo[1], a.hi[1], c.lo[1], c.hi[1]) > tol &&
					overlap(a.lo[2], a.hi[2], c.lo[2], c.hi[2]) > tol {
					fail("%s overlaps %s", it.ID, b.Items[j].ID)
				}
			}
			got := []float64{a.hi[0] - a.lo[0], a.hi[1] - a.lo[1], a.hi[2] - a.lo[2]}
			want := []float64{s.w, s.h, s.d}
			sort.Float64s(got)
			sort.Float64s(want)
			for k := range 3 {
				if math.Abs(got[k]-want[k]) > tol {
					fail("%s has the wrong size", it.ID)
				}
			}
			if s.upright && math.Abs((a.hi[1]-a.lo[1])-s.h) > tol {
				fail("%s was tipped over", it.ID)
			}
			if s.loadBear > 0 && ld[i] > s.loadBear+tol {
				fail("%s carries %.2f kg, limit %.2f", it.ID, ld[i], s.loadBear)
			}
			supported := 0.0
			for j, o := range b.Items {
				if j == i {
					continue
				}
				c := contact(boxOf(o), a)
				if s.fragile && c > tol {
					fail("%s rests on fragile %s", o.ID, it.ID)
				}
				supported += contact(a, boxOf(o))
			}
			base := (a.hi[0] - a.lo[0]) * (a.hi[2] - a.lo[2])
			if stability > 0 && a.lo[1] > tol && supported/base < stability-tol {
				fail("%s has %.0f%% of its base supported", it.ID, 100*supported/base)
			}
		}
		if !b.AllowsWeight(weight) {
			fail("%s carries %.2f kg, limit %.2f", b.ID, weight, b.MaxWeight)
		}
	}
	if res.Stats.FittedItems+res.Stats.UnfittedCount != len(sc.items) || res.Stats.UnfittedCount != len(res.UnfittedItems) {
		fail("stats %+v do not match the result", res.Stats)
	}
	return errs
}

func checkScenario(t *testing.T, seed uint64) {
	t.Helper()
	for _, ep := range entryPointsWithStability() {
		sc := randomScenario(seed)
		bins, items := sc.bins, sc.items
		if ep.fixed {
			bins = fixedStock(bins)
		}
		if strings.HasPrefix(ep.name, "BranchBound") && len(items) > 6 {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		res, err := ep.run(ctx, bins, items)
		cancel()
		if err != nil {
			t.Fatalf("seed %d, %s: %v", seed, ep.name, err)
		}
		for _, e := range physicalErrors(sc, res, ep.stability) {
			t.Errorf("seed %d, %s: %s", seed, ep.name, e)
		}
	}
}

func TestPhysicalRulesHoldOnRandomScenarios(t *testing.T) {
	n := uint64(40)
	if testing.Short() {
		n = 8
	}
	for seed := range n {
		checkScenario(t, seed)
	}
}

// FuzzPhysicalRules explores more scenarios with go test -fuzz=FuzzPhysicalRules.
func FuzzPhysicalRules(f *testing.F) {
	for _, seed := range []uint64{1, 4, 15, 49} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed uint64) {
		checkScenario(t, seed)
	})
}

type ruleRun struct {
	name      string
	fixed     bool // packs into stock bins rather than a catalog of types
	stability float64
	run       func(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error)
}

// entryPointsWithStability lists every solver with every engine, with and
// without a support ratio.
func entryPointsWithStability() []ruleRun {
	engines := []struct {
		name string
		new  func(ratio float64) func() placement.Engine
	}{
		{"Pivot", func(r float64) func() placement.Engine {
			return func() placement.Engine { return placement.NewPivotEngine(placement.WithStability(r)) }
		}},
		{"ExtremePoint", func(r float64) func() placement.Engine {
			return func() placement.Engine { return placement.NewExtremePointEngine(placement.WithEPStability(r)) }
		}},
		{"MaxRects", func(r float64) func() placement.Engine {
			return func() placement.Engine { return placement.NewMaxRectsEngine(placement.WithMaxRectsStability(r)) }
		}},
		{"LAFF", func(r float64) func() placement.Engine {
			return func() placement.Engine { return placement.NewLAFFEngine(placement.WithLAFFStability(r)) }
		}},
		{"LAFFFast", func(r float64) func() placement.Engine {
			return func() placement.Engine {
				return placement.NewLAFFEngine(placement.LAFFFast(), placement.WithLAFFStability(r))
			}
		}},
	}
	var out []ruleRun
	for _, e := range engines {
		for _, ratio := range []float64{0, 0.6} {
			newEngine := e.new(ratio)
			suffix := fmt.Sprintf("/%s/support%.0f%%", e.name, ratio*100)
			out = append(out,
				ruleRun{"Pack" + suffix, false, ratio, func(ctx context.Context, b []*model.Bin, i []*model.Item) (*model.Result, error) {
					return gopackx.Pack(ctx, b, i, gopackx.WithEngine(newEngine))
				}},
				ruleRun{"Metaheuristic" + suffix, false, ratio, solver.NewMetaheuristic(newEngine, solver.MetaMaxIter(20)).Solve},
				ruleRun{"TrialPacking" + suffix, false, ratio, solver.NewTrialPacking(newEngine, solver.WithLookahead()).Solve},
				ruleRun{"Packer" + suffix, true, ratio, func(ctx context.Context, b []*model.Bin, i []*model.Item) (*model.Result, error) {
					p := packer.NewPacker(packer.WithPlacementEngine(newEngine()))
					for _, x := range b {
						p.AddBin(x)
					}
					for _, x := range i {
						p.AddItem(x)
					}
					return p.Pack(ctx)
				}},
				ruleRun{"BranchBound" + suffix, true, ratio, solver.NewBranchBound(newEngine).Solve},
				ruleRun{"Parallel" + suffix, true, ratio, solver.NewParallel(solver.WithConfig(newEngine, strategy.BestFitDecreasing)).Solve},
			)
		}
	}
	return out
}

// fixedStock turns box types into two boxes of each type in stock.
func fixedStock(types []*model.Bin) []*model.Bin {
	var out []*model.Bin
	for _, t := range types {
		for k := range 2 {
			b := t.CloneEmpty()
			b.ID = t.ID + "#" + strconv.Itoa(k)
			out = append(out, b)
		}
	}
	return out
}
