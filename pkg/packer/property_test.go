package packer_test

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/solver"
	"github.com/jcoruiz/gopackx/pkg/stability"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// ---------- helpers ----------

type engineSpec struct {
	name string
	new  func() placement.Engine
}

type engineStabilitySpec struct {
	name string
	new  func() placement.Engine
}

var engines = []engineSpec{
	{"Pivot", func() placement.Engine { return placement.NewPivotEngine() }},
	{"ExtremePoint", func() placement.Engine { return placement.NewExtremePointEngine() }},
	{"LAFF", func() placement.Engine { return placement.NewLAFFEngine() }},
	{"LAFFFast", func() placement.Engine { return placement.NewLAFFEngine(placement.LAFFFast()) }},
	{"MaxRects", func() placement.Engine { return placement.NewMaxRectsEngine() }},
}

var enginesWithStability = []engineStabilitySpec{
	{"Pivot-S70", func() placement.Engine { return placement.NewPivotEngine(placement.WithStability(0.7)) }},
	{"EP-S70", func() placement.Engine { return placement.NewExtremePointEngine(placement.WithEPStability(0.7)) }},
	{"LAFF-S70", func() placement.Engine { return placement.NewLAFFEngine(placement.WithLAFFStability(0.7)) }},
	{"MaxRects-S70", func() placement.Engine { return placement.NewMaxRectsEngine(placement.WithMaxRectsStability(0.7)) }},
}

var strategies = []strategy.Type{
	strategy.MinimizeBins,
	strategy.Greedy,
	strategy.BestFit,
	strategy.BestFitDecreasing,
	strategy.NextFit,
	strategy.WorstFit,
	strategy.AlmostWorstFit,
}

func strategyName(st strategy.Type) string {
	names := map[strategy.Type]string{
		strategy.MinimizeBins:     "MinimizeBins",
		strategy.Greedy:           "Greedy",
		strategy.BestFit:          "BestFit",
		strategy.BestFitDecreasing: "BestFitDecreasing",
		strategy.NextFit:          "NextFit",
		strategy.WorstFit:         "WorstFit",
		strategy.AlmostWorstFit:   "AlmostWorstFit",
	}
	return names[st]
}

// randItems generates n items with random dimensions within [minD, maxD] and weight [1, maxW].
func randItems(rng *rand.Rand, n int, minD, maxD, maxW float64, opts ...model.ItemOption) []*model.Item {
	items := make([]*model.Item, n)
	for i := range items {
		w := minD + rng.Float64()*(maxD-minD)
		h := minD + rng.Float64()*(maxD-minD)
		d := minD + rng.Float64()*(maxD-minD)
		wt := 1 + rng.Float64()*(maxW-1)
		items[i] = model.NewItem(fmt.Sprintf("i%d", i), w, h, d, wt, opts...)
	}
	return items
}

const epsilon = 1e-6

// ---------- invariant checkers ----------

// aabbOverlap checks if two items overlap in 3D using direct AABB comparison
// with epsilon tolerance. This avoids false positives from floating-point
// rounding in center-distance methods when items are exactly touching.
func aabbOverlap(a, b *model.Item) bool {
	da := a.Dimension()
	db := b.Dimension()
	for axis := 0; axis < 3; axis++ {
		aMin := a.Position[axis]
		aMax := aMin + da[axis]
		bMin := b.Position[axis]
		bMax := bMin + db[axis]
		if aMin >= bMax-epsilon || bMin >= aMax-epsilon {
			return false // separated on this axis
		}
	}
	return true
}

func checkNoIntersections(t *testing.T, bins []*model.Bin) {
	t.Helper()
	for _, bin := range bins {
		for i := 0; i < len(bin.Items); i++ {
			for j := i + 1; j < len(bin.Items); j++ {
				if aabbOverlap(bin.Items[i], bin.Items[j]) {
					t.Errorf("bin %s: items %d (%s) and %d (%s) intersect\n  pos1=%v dim1=%v\n  pos2=%v dim2=%v",
						bin.ID, i, bin.Items[i].ID, j, bin.Items[j].ID,
						bin.Items[i].Position, bin.Items[i].Dimension(),
						bin.Items[j].Position, bin.Items[j].Dimension())
				}
			}
		}
	}
}

func checkWithinBounds(t *testing.T, bins []*model.Bin) {
	t.Helper()
	for _, bin := range bins {
		for i, item := range bin.Items {
			dim := item.Dimension()
			if item.Position[0]+dim[0] > bin.Width+epsilon ||
				item.Position[1]+dim[1] > bin.Height+epsilon ||
				item.Position[2]+dim[2] > bin.Depth+epsilon {
				t.Errorf("bin %s: item %d (%s) out of bounds pos=%v dim=%v bin=[%.0f,%.0f,%.0f]",
					bin.ID, i, item.ID, item.Position, dim, bin.Width, bin.Height, bin.Depth)
			}
			if item.Position[0] < -epsilon || item.Position[1] < -epsilon || item.Position[2] < -epsilon {
				t.Errorf("bin %s: item %d (%s) has negative position %v",
					bin.ID, i, item.ID, item.Position)
			}
		}
	}
}

func checkWeightLimits(t *testing.T, bins []*model.Bin) {
	t.Helper()
	for _, bin := range bins {
		totalW := 0.0
		for _, item := range bin.Items {
			totalW += item.Weight
		}
		if totalW > bin.MaxWeight+epsilon {
			t.Errorf("bin %s: total weight %.2f exceeds max %.2f",
				bin.ID, totalW, bin.MaxWeight)
		}
	}
}

func checkFragile(t *testing.T, bins []*model.Bin) {
	t.Helper()
	for _, bin := range bins {
		for _, fragile := range bin.Items {
			if !fragile.Fragile {
				continue
			}
			fDim := fragile.Dimension()
			fTop := fragile.Position[1] + fDim[1]

			for _, other := range bin.Items {
				if other == fragile {
					continue
				}
				// Check if other item is resting directly on top of fragile.
				if math.Abs(other.Position[1]-fTop) > epsilon {
					continue
				}
				oDim := other.Dimension()
				overlapW := overlapLen(other.Position[0], oDim[0], fragile.Position[0], fDim[0])
				overlapD := overlapLen(other.Position[2], oDim[2], fragile.Position[2], fDim[2])
				if overlapW > epsilon && overlapD > epsilon {
					t.Errorf("bin %s: item %s placed on top of fragile item %s",
						bin.ID, other.ID, fragile.ID)
				}
			}
		}
	}
}

func checkUpright(t *testing.T, bins []*model.Bin) {
	t.Helper()
	for _, bin := range bins {
		for _, item := range bin.Items {
			if len(item.AllowedRotations) != 2 {
				continue // not upright-constrained
			}
			rt := item.RotationType
			if rt != model.RotationWHD && rt != model.RotationDHW {
				t.Errorf("bin %s: upright item %s has rotation %d (expected WHD or DHW)",
					bin.ID, item.ID, rt)
			}
		}
	}
}

func checkLoadBearing(t *testing.T, bins []*model.Bin) {
	t.Helper()
	for _, bin := range bins {
		for _, item := range bin.Items {
			if item.LoadBear <= 0 {
				continue
			}
			above := stability.WeightAbove(item, bin.Items)
			if above > item.LoadBear+epsilon {
				t.Errorf("bin %s: item %s has %.2f weight above, load-bearing limit is %.2f",
					bin.ID, item.ID, above, item.LoadBear)
			}
		}
	}
}

func checkConservation(t *testing.T, result *model.Result, totalItems int) {
	t.Helper()
	fitted := 0
	for _, bin := range result.Bins {
		fitted += len(bin.Items)
	}
	unfitted := len(result.UnfittedItems)
	if fitted+unfitted != totalItems {
		t.Errorf("conservation violation: fitted(%d) + unfitted(%d) = %d, want %d",
			fitted, unfitted, fitted+unfitted, totalItems)
	}
}

func checkStatsConsistency(t *testing.T, result *model.Result, totalItems int) {
	t.Helper()
	fitted := 0
	for _, bin := range result.Bins {
		fitted += len(bin.Items)
	}
	if result.Stats.FittedItems != fitted {
		t.Errorf("stats: FittedItems=%d but actual fitted=%d", result.Stats.FittedItems, fitted)
	}
	if result.Stats.UnfittedCount != len(result.UnfittedItems) {
		t.Errorf("stats: UnfittedCount=%d but actual unfitted=%d",
			result.Stats.UnfittedCount, len(result.UnfittedItems))
	}
	if result.Stats.TotalItems != totalItems {
		t.Errorf("stats: TotalItems=%d but expected %d", result.Stats.TotalItems, totalItems)
	}
	activeBins := 0
	for _, bin := range result.Bins {
		if len(bin.Items) > 0 {
			activeBins++
		}
	}
	if result.Stats.TotalBins != activeBins {
		t.Errorf("stats: TotalBins=%d but active bins=%d", result.Stats.TotalBins, activeBins)
	}
}

func checkAllInvariants(t *testing.T, result *model.Result, totalItems int) {
	t.Helper()
	checkNoIntersections(t, result.Bins)
	checkWithinBounds(t, result.Bins)
	checkWeightLimits(t, result.Bins)
	checkFragile(t, result.Bins)
	checkUpright(t, result.Bins)
	checkConservation(t, result, totalItems)
	checkStatsConsistency(t, result, totalItems)
}

func overlapLen(pos1, len1, pos2, len2 float64) float64 {
	start := math.Max(pos1, pos2)
	end := math.Min(pos1+len1, pos2+len2)
	if end <= start {
		return 0
	}
	return end - start
}

// ---------- property tests ----------

// TestProperty_AllEnginesAllStrategies runs every engine×strategy combination
// with random items and verifies all invariants hold.
func TestProperty_AllEnginesAllStrategies(t *testing.T) {
	seeds := []int64{42, 123, 999, 7, 2024}

	for _, eng := range engines {
		for _, st := range strategies {
			for _, seed := range seeds {
				name := fmt.Sprintf("%s/%s/seed%d", eng.name, strategyName(st), seed)
				t.Run(name, func(t *testing.T) {
					rng := rand.New(rand.NewSource(seed))
					nItems := 10 + rng.Intn(30)

					p := packer.NewPacker(
						packer.WithPlacementEngine(eng.new()),
						packer.WithStrategy(st),
					)
					p.AddBin(model.NewBin("b1", 100, 100, 100, 500))
					p.AddBin(model.NewBin("b2", 80, 60, 80, 300))

					items := randItems(rng, nItems, 5, 40, 20)
					for _, item := range items {
						p.AddItem(item)
					}

					result, err := p.Pack(context.Background())
					if err != nil {
						t.Fatalf("Pack() error: %v", err)
					}

					checkAllInvariants(t, result, nItems)
				})
			}
		}
	}
}

// TestProperty_ManySmallItemsTightBin verifies behavior when many small items
// are packed into bins that are just barely large enough.
func TestProperty_ManySmallItemsTightBin(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(314))

			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			p.AddBin(model.NewBin("tight", 30, 30, 30, 1000))

			nItems := 50
			items := randItems(rng, nItems, 2, 10, 5)
			for _, item := range items {
				p.AddItem(item)
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatalf("Pack() error: %v", err)
			}

			checkAllInvariants(t, result, nItems)
		})
	}
}

// TestProperty_IdenticalItems packs identical cubes — a case where packing
// is geometrically deterministic.
func TestProperty_IdenticalItems(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			p.AddBin(model.NewBin("box", 20, 20, 20, 1000))

			nItems := 8
			for i := 0; i < nItems; i++ {
				p.AddItem(model.NewItem(fmt.Sprintf("cube%d", i), 10, 10, 10, 1))
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)

			// 8 cubes of 10³ fit exactly in a 20³ bin.
			if result.Stats.FittedItems != 8 {
				t.Errorf("%s: FittedItems=%d, want 8 (perfect fit)", eng.name, result.Stats.FittedItems)
			}
		})
	}
}

// TestProperty_FragileItems verifies fragile constraints across engines.
func TestProperty_FragileItems(t *testing.T) {
	seeds := []int64{1, 50, 200}

	for _, eng := range engines {
		for _, seed := range seeds {
			name := fmt.Sprintf("%s/seed%d", eng.name, seed)
			t.Run(name, func(t *testing.T) {
				rng := rand.New(rand.NewSource(seed))

				p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
				p.AddBin(model.NewBin("box", 50, 50, 50, 1000))

				nItems := 15
				for i := 0; i < nItems; i++ {
					w := 5 + rng.Float64()*15
					h := 5 + rng.Float64()*15
					d := 5 + rng.Float64()*15
					opts := []model.ItemOption{}
					if rng.Float64() < 0.3 {
						opts = append(opts, model.ItemFragile())
					}
					p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), w, h, d, 1, opts...))
				}

				result, err := p.Pack(context.Background())
				if err != nil {
					t.Fatal(err)
				}

				checkAllInvariants(t, result, nItems)
			})
		}
	}
}

// TestProperty_UprightItems verifies upright constraints across engines.
func TestProperty_UprightItems(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(77))

			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			p.AddBin(model.NewBin("box", 60, 60, 60, 1000))

			nItems := 20
			for i := 0; i < nItems; i++ {
				w := 5 + rng.Float64()*20
				h := 5 + rng.Float64()*20
				d := 5 + rng.Float64()*20
				opts := []model.ItemOption{}
				if rng.Float64() < 0.5 {
					opts = append(opts, model.ItemUpright())
				}
				p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), w, h, d, 1, opts...))
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)
		})
	}
}

// TestProperty_WeightConstrained tests with low weight capacity bins
// to exercise weight filtering paths.
func TestProperty_WeightConstrained(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(256))

			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			// Bin has plenty of volume but very limited weight.
			p.AddBin(model.NewBin("light-bin", 100, 100, 100, 25))

			nItems := 20
			items := randItems(rng, nItems, 5, 20, 10)
			for _, item := range items {
				p.AddItem(item)
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)

			// Verify total weight in each bin is within limit.
			for _, bin := range result.Bins {
				totalW := 0.0
				for _, item := range bin.Items {
					totalW += item.Weight
				}
				if totalW > 25+epsilon {
					t.Errorf("bin %s total weight %.2f exceeds limit 25", bin.ID, totalW)
				}
			}
		})
	}
}

// TestProperty_StabilityConstraints verifies load-bearing and support
// when stability is enabled.
func TestProperty_StabilityConstraints(t *testing.T) {
	seeds := []int64{10, 42, 99}

	for _, eng := range enginesWithStability {
		for _, seed := range seeds {
			name := fmt.Sprintf("%s/seed%d", eng.name, seed)
			t.Run(name, func(t *testing.T) {
				rng := rand.New(rand.NewSource(seed))

				p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
				p.AddBin(model.NewBin("box", 60, 60, 60, 1000))

				nItems := 15
				for i := 0; i < nItems; i++ {
					w := 5 + rng.Float64()*20
					h := 5 + rng.Float64()*15
					d := 5 + rng.Float64()*20
					wt := 1 + rng.Float64()*10
					opts := []model.ItemOption{}
					if rng.Float64() < 0.3 {
						opts = append(opts, model.ItemLoadBear(5+rng.Float64()*20))
					}
					p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), w, h, d, wt, opts...))
				}

				result, err := p.Pack(context.Background())
				if err != nil {
					t.Fatal(err)
				}

				checkAllInvariants(t, result, nItems)
				checkLoadBearing(t, result.Bins)
			})
		}
	}
}

// TestProperty_MixedConstraints combines fragile, upright, load-bearing,
// and priority constraints in a single packing run.
func TestProperty_MixedConstraints(t *testing.T) {
	seeds := []int64{11, 55, 333}

	for _, eng := range enginesWithStability {
		for _, seed := range seeds {
			name := fmt.Sprintf("%s/seed%d", eng.name, seed)
			t.Run(name, func(t *testing.T) {
				rng := rand.New(rand.NewSource(seed))

				p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
				p.AddBin(model.NewBin("box", 80, 80, 80, 500))

				nItems := 20
				for i := 0; i < nItems; i++ {
					w := 5 + rng.Float64()*25
					h := 5 + rng.Float64()*20
					d := 5 + rng.Float64()*25
					wt := 1 + rng.Float64()*15

					var opts []model.ItemOption
					r := rng.Float64()
					if r < 0.15 {
						opts = append(opts, model.ItemFragile())
					} else if r < 0.3 {
						opts = append(opts, model.ItemUpright())
					}
					if rng.Float64() < 0.3 {
						opts = append(opts, model.ItemLoadBear(10+rng.Float64()*30))
					}
					if rng.Float64() < 0.2 {
						opts = append(opts, model.ItemPriority(1+rng.Intn(3)))
					}

					p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), w, h, d, wt, opts...))
				}

				result, err := p.Pack(context.Background())
				if err != nil {
					t.Fatal(err)
				}

				checkAllInvariants(t, result, nItems)
				checkLoadBearing(t, result.Bins)
			})
		}
	}
}

// TestProperty_MultipleBins verifies packing across many bins.
func TestProperty_MultipleBins(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(42))

			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			for i := 0; i < 5; i++ {
				w := 30 + rng.Float64()*70
				h := 30 + rng.Float64()*70
				d := 30 + rng.Float64()*70
				p.AddBin(model.NewBin(fmt.Sprintf("bin%d", i), w, h, d, 200))
			}

			nItems := 40
			items := randItems(rng, nItems, 5, 25, 15)
			for _, item := range items {
				p.AddItem(item)
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)
		})
	}
}

// TestProperty_ExtremeAspectRatios tests items with very elongated shapes
// (e.g. long thin rods, flat panels) that stress rotation handling.
func TestProperty_ExtremeAspectRatios(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(161))

			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
			p.AddBin(model.NewBin("box", 100, 100, 100, 1000))

			nItems := 15
			for i := 0; i < nItems; i++ {
				var w, h, d float64
				switch rng.Intn(3) {
				case 0: // thin rod
					w, h, d = 2+rng.Float64()*3, 40+rng.Float64()*50, 2+rng.Float64()*3
				case 1: // flat panel
					w, h, d = 20+rng.Float64()*30, 2+rng.Float64()*3, 20+rng.Float64()*30
				case 2: // normal
					w, h, d = 10+rng.Float64()*20, 10+rng.Float64()*20, 10+rng.Float64()*20
				}
				p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), w, h, d, 1))
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)
		})
	}
}

// TestProperty_SingleItemPerBin verifies that items too large to share
// bins are handled correctly.
func TestProperty_SingleItemPerBin(t *testing.T) {
	for _, eng := range engines {
		t.Run(eng.name, func(t *testing.T) {
			p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))

			for i := 0; i < 5; i++ {
				p.AddBin(model.NewBin(fmt.Sprintf("bin%d", i), 10, 10, 10, 100))
			}
			// Each item fills an entire bin.
			nItems := 5
			for i := 0; i < nItems; i++ {
				p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), 10, 10, 10, 1))
			}

			result, err := p.Pack(context.Background())
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)
			if result.Stats.FittedItems != 5 {
				t.Errorf("FittedItems=%d, want 5", result.Stats.FittedItems)
			}
		})
	}
}

// ---------- cross-engine consistency ----------

// TestCrossEngine_AllProduceValidPackings verifies that the same input
// produces valid (but possibly different) packings across all engines.
func TestCrossEngine_AllProduceValidPackings(t *testing.T) {
	seeds := []int64{42, 100, 777}

	for _, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		nItems := 20 + rng.Intn(20)

		// Generate items once, reuse across engines.
		type itemSpec struct {
			id     string
			w, h, d, wt float64
		}
		specs := make([]itemSpec, nItems)
		for i := range specs {
			specs[i] = itemSpec{
				id: fmt.Sprintf("i%d", i),
				w:  5 + rng.Float64()*30,
				h:  5 + rng.Float64()*30,
				d:  5 + rng.Float64()*30,
				wt: 1 + rng.Float64()*10,
			}
		}

		for _, eng := range engines {
			name := fmt.Sprintf("seed%d/%s", seed, eng.name)
			t.Run(name, func(t *testing.T) {
				p := packer.NewPacker(packer.WithPlacementEngine(eng.new()))
				p.AddBin(model.NewBin("b1", 100, 100, 100, 500))

				for _, s := range specs {
					p.AddItem(model.NewItem(s.id, s.w, s.h, s.d, s.wt))
				}

				result, err := p.Pack(context.Background())
				if err != nil {
					t.Fatalf("Pack() error: %v", err)
				}

				checkAllInvariants(t, result, nItems)
			})
		}
	}
}

// ---------- solver property tests ----------

// TestProperty_SolverBranchBound verifies B&B invariants with random inputs.
func TestProperty_SolverBranchBound(t *testing.T) {
	seeds := []int64{42, 99, 256}

	for _, seed := range seeds {
		name := fmt.Sprintf("seed%d", seed)
		t.Run(name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			nItems := 6 + rng.Intn(4) // keep small for B&B

			bins := []*model.Bin{model.NewBin("box", 40, 40, 40, 500)}
			items := randItems(rng, nItems, 5, 20, 10)

			bb := solver.NewBranchBound(func() placement.Engine {
				return placement.NewPivotEngine()
			})

			result, err := bb.Solve(context.Background(), bins, items)
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)
		})
	}
}

// TestProperty_SolverParallel verifies Parallel solver invariants.
func TestProperty_SolverParallel(t *testing.T) {
	seeds := []int64{42, 99, 500}

	for _, seed := range seeds {
		name := fmt.Sprintf("seed%d", seed)
		t.Run(name, func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			nItems := 15 + rng.Intn(20)

			bins := []*model.Bin{
				model.NewBin("b1", 80, 80, 80, 500),
				model.NewBin("b2", 60, 60, 60, 300),
			}
			items := randItems(rng, nItems, 5, 25, 10)

			ps := solver.NewParallel()
			result, err := ps.Solve(context.Background(), bins, items)
			if err != nil {
				t.Fatal(err)
			}

			checkAllInvariants(t, result, nItems)
		})
	}
}

// TestProperty_SolverAtLeastAsGoodAsGreedy verifies that solvers never
// produce worse results than a simple greedy packing.
func TestProperty_SolverAtLeastAsGoodAsGreedy(t *testing.T) {
	seeds := []int64{42, 88, 200}

	for _, seed := range seeds {
		rng := rand.New(rand.NewSource(seed))
		nItems := 8 + rng.Intn(5)

		type itemSpec struct {
			w, h, d, wt float64
		}
		specs := make([]itemSpec, nItems)
		for i := range specs {
			specs[i] = itemSpec{
				w:  5 + rng.Float64()*20,
				h:  5 + rng.Float64()*20,
				d:  5 + rng.Float64()*20,
				wt: 1 + rng.Float64()*5,
			}
		}
		binW, binH, binD, binMW := 50.0, 50.0, 50.0, 500.0

		// Greedy baseline.
		gp := packer.NewPacker()
		gp.AddBin(model.NewBin("box", binW, binH, binD, binMW))
		for i, s := range specs {
			gp.AddItem(model.NewItem(fmt.Sprintf("i%d", i), s.w, s.h, s.d, s.wt))
		}
		greedyResult, _ := gp.Pack(context.Background())

		// B&B solver.
		bbBins := []*model.Bin{model.NewBin("box", binW, binH, binD, binMW)}
		bbItems := make([]*model.Item, nItems)
		for i, s := range specs {
			bbItems[i] = model.NewItem(fmt.Sprintf("i%d", i), s.w, s.h, s.d, s.wt)
		}
		bb := solver.NewBranchBound(func() placement.Engine { return placement.NewPivotEngine() })
		bbResult, _ := bb.Solve(context.Background(), bbBins, bbItems)

		// Parallel solver.
		psBins := []*model.Bin{model.NewBin("box", binW, binH, binD, binMW)}
		psItems := make([]*model.Item, nItems)
		for i, s := range specs {
			psItems[i] = model.NewItem(fmt.Sprintf("i%d", i), s.w, s.h, s.d, s.wt)
		}
		ps := solver.NewParallel()
		psResult, _ := ps.Solve(context.Background(), psBins, psItems)

		name := fmt.Sprintf("seed%d", seed)
		t.Run(name, func(t *testing.T) {
			if bbResult.Stats.FittedItems < greedyResult.Stats.FittedItems {
				t.Errorf("B&B (%d) worse than greedy (%d)",
					bbResult.Stats.FittedItems, greedyResult.Stats.FittedItems)
			}
			if psResult.Stats.FittedItems < greedyResult.Stats.FittedItems {
				t.Errorf("Parallel (%d) worse than greedy (%d)",
					psResult.Stats.FittedItems, greedyResult.Stats.FittedItems)
			}
			t.Logf("greedy=%d, B&B=%d, parallel=%d",
				greedyResult.Stats.FittedItems, bbResult.Stats.FittedItems, psResult.Stats.FittedItems)
		})
	}
}

// ---------- fuzz test ----------

func FuzzPack(f *testing.F) {
	// Seed corpus.
	f.Add(uint32(42), uint8(5), uint8(50), uint8(50), uint8(50))
	f.Add(uint32(0), uint8(1), uint8(10), uint8(10), uint8(10))
	f.Add(uint32(99), uint8(20), uint8(30), uint8(30), uint8(30))

	f.Fuzz(func(t *testing.T, seed uint32, nItemsRaw uint8, binW, binH, binD uint8) {
		nItems := int(nItemsRaw%30) + 1
		bw := float64(binW%100) + 10
		bh := float64(binH%100) + 10
		bd := float64(binD%100) + 10

		rng := rand.New(rand.NewSource(int64(seed)))

		p := packer.NewPacker()
		p.AddBin(model.NewBin("box", bw, bh, bd, 10000))

		for i := 0; i < nItems; i++ {
			w := 1 + rng.Float64()*(bw/2)
			h := 1 + rng.Float64()*(bh/2)
			d := 1 + rng.Float64()*(bd/2)
			p.AddItem(model.NewItem(fmt.Sprintf("i%d", i), w, h, d, 1))
		}

		result, err := p.Pack(context.Background())
		if err != nil {
			t.Fatal(err)
		}

		checkAllInvariants(t, result, nItems)
	})
}
