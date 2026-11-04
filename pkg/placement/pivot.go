package placement

import (
	"slices"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*PivotEngine)(nil)
var _ Resetter = (*PivotEngine)(nil)

// PivotEngine places items using pivot points generated from corners of placed items.
//
// Pivots are tried lowest first (Y, then Z, then X), so an item goes to the
// floor or the lowest free level before it is stacked on top of other items.
type PivotEngine struct {
	enableStability bool
	supportRatio    float64

	// Sorted pivots of the bin used last and the bin revision they match.
	// The pivots of other bins are kept in saved.
	bin    *model.Bin
	rev    uint64
	pivots [][3]float64
	saved  binStates[[][3]float64]
}

// PivotOption configures the PivotEngine.
type PivotOption func(*PivotEngine)

// WithStability enables stability checking with the given support ratio threshold.
func WithStability(ratio float64) PivotOption {
	return func(e *PivotEngine) {
		e.enableStability = true
		e.supportRatio = ratio
	}
}

// NewPivotEngine creates a new PivotEngine with the given options.
func NewPivotEngine(opts ...PivotOption) *PivotEngine {
	e := &PivotEngine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// PlaceItem attempts to place an item in the bin using pivot point generation.
func (e *PivotEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	origRT := item.RotationType
	origPos := item.Position

	if e.bin != bin || e.rev != bin.Revision() {
		e.switchBin(bin)
	}
	pivots := e.pivots
	rotations := rotation.AllowedFor(item)

	// Pre-compute dimensions for each rotation and dedup identical dimensions.
	type rotDim struct {
		rt  model.RotationType
		dim [3]float64
	}
	var rds [6]rotDim
	nRot := 0
	for _, rt := range rotations {
		dim := rotation.DimensionsFor(item, rt)
		dup := false
		for j := range nRot {
			if rds[j].dim == dim {
				dup = true
				break
			}
		}
		if !dup {
			rds[nRot] = rotDim{rt, dim}
			nRot++
		}
	}

	bw, bh, bd := bin.Width+epsilon, bin.Height+epsilon, bin.Depth+epsilon
	stab := e.enableStability
	ratio := e.supportRatio

	// Conflict-driven rejection: track recent blockers' AABBs on stack.
	const maxBlockers = 4
	var blockers [maxBlockers][6]float64
	nBlockers := 0
	writeIdx := 0

	for _, pivot := range pivots {
		for ri := range nRot {
			dim := rds[ri].dim

			px1 := pivot[0] + dim[0]
			py1 := pivot[1] + dim[1]
			pz1 := pivot[2] + dim[2]
			if px1 > bw || py1 > bh || pz1 > bd {
				continue
			}

			// Conflict-driven pre-rejection from stack-cached blockers.
			blocked := false
			for bi := range nBlockers {
				b := &blockers[bi]
				if pivot[0] < b[3]-epsilon && b[0] < px1-epsilon &&
					pivot[1] < b[4]-epsilon && b[1] < py1-epsilon &&
					pivot[2] < b[5]-epsilon && b[2] < pz1-epsilon {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}

			item.RotationType = rds[ri].rt
			item.Position = pivot

			blocker := canPlaceDimBlocker(bin, item, dim, stab, ratio)
			if blocker >= 0 {
				lo, hi := bin.Box(blocker)
				blockers[writeIdx] = [6]float64{lo[0], lo[1], lo[2], hi[0], hi[1], hi[2]}
				writeIdx = (writeIdx + 1) & (maxBlockers - 1)
				if nBlockers < maxBlockers {
					nBlockers++
				}
				continue
			}
			if blocker == -2 {
				continue
			}

			// Success! Try fix-point correction.
			savedPos := item.Position
			fixPointDim(bin, item, dim)
			if item.Position != savedPos {
				if !canPlaceDim(bin, item, dim, stab, ratio) {
					item.Position = savedPos
				}
			}

			bin.PlaceItem(item)
			e.addPivots(bin.Box(len(bin.Items) - 1))
			e.rev = bin.Revision()
			return true
		}
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

// Reset forgets the pivots kept for every bin. Call it before reusing the
// engine for an unrelated packing run; packer.Packer does it on each Pack.
func (e *PivotEngine) Reset() {
	e.bin, e.rev, e.pivots = nil, 0, nil
	e.saved.reset()
}

// switchBin saves the pivots of the current bin and restores those of bin,
// rebuilding them only if this engine has not seen bin in its current state.
func (e *PivotEngine) switchBin(bin *model.Bin) {
	if e.bin != nil && e.bin != bin {
		e.saved.save(e.bin, e.pivots, e.rev)
		// The buffer now belongs to the saved state.
		e.pivots = nil
	}
	if pts, ok := e.saved.load(bin); ok {
		e.bin, e.rev, e.pivots = bin, bin.Revision(), pts
		return
	}

	// Build the pivots of bin from scratch: the origin plus the points to the
	// right of, on top of and behind every placed item, sorted and without
	// duplicates.
	n := len(bin.Items)
	if need := 1 + 3*n; cap(e.pivots) < need {
		e.pivots = make([][3]float64, 0, 2*need)
	}
	e.pivots = append(e.pivots[:0], [3]float64{0, 0, 0})
	for k := range n {
		lo, hi := bin.Box(k)
		e.pivots = append(e.pivots,
			[3]float64{hi[0], lo[1], lo[2]},
			[3]float64{lo[0], hi[1], lo[2]},
			[3]float64{lo[0], lo[1], hi[2]},
		)
	}
	slices.SortFunc(e.pivots, comparePivots)
	e.pivots = slices.Compact(e.pivots)
	e.bin, e.rev = bin, bin.Revision()
}

// addPivots inserts the pivots of an item placed at lo..hi, keeping the
// pivots sorted and without duplicates.
func (e *PivotEngine) addPivots(lo, hi [3]float64) {
	for _, p := range [3][3]float64{
		{hi[0], lo[1], lo[2]},
		{lo[0], hi[1], lo[2]},
		{lo[0], lo[1], hi[2]},
	} {
		i, found := slices.BinarySearchFunc(e.pivots, p, comparePivots)
		if !found {
			e.pivots = slices.Insert(e.pivots, i, p)
		}
	}
}

// comparePivots orders pivots bottom to top (Y), then front to back (Z), then
// left to right (X).
func comparePivots(a, b [3]float64) int {
	for _, axis := range [3]model.Axis{model.HeightAxis, model.DepthAxis, model.WidthAxis} {
		switch {
		case a[axis] < b[axis]:
			return -1
		case a[axis] > b[axis]:
			return 1
		}
	}
	return 0
}
