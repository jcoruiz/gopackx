package placement

import (
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*PivotEngine)(nil)
var _ Resetter = (*PivotEngine)(nil)

// PivotEngine places items using pivot points generated from corners of placed items.
type PivotEngine struct {
	enableStability bool
	supportRatio    float64
	pivotBuf        [][3]float64
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

	pivots := e.generatePivots(bin)
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

	// Conflict-driven rejection: candidates are first tested against the
	// last items that blocked one.
	var bl blockers

	for _, pivot := range pivots {
		for ri := range nRot {
			dim := rds[ri].dim

			px1 := pivot[0] + dim[0]
			py1 := pivot[1] + dim[1]
			pz1 := pivot[2] + dim[2]
			if px1 > bw || py1 > bh || pz1 > bd {
				continue
			}

			if bl.hit(pivot, dim) {
				continue
			}

			item.RotationType = rds[ri].rt
			item.Position = pivot

			blocker := canPlaceDimBlocker(bin, item, dim, stab, ratio)
			if blocker >= 0 {
				bl.add(bin, blocker)
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
			return true
		}
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

// Reset is a no-op: the pivot engine keeps no per-bin state.
func (e *PivotEngine) Reset() {}

// generatePivots returns candidate positions from corners of placed items.
func (e *PivotEngine) generatePivots(bin *model.Bin) [][3]float64 {
	needed := 1 + 3*len(bin.Items)
	if cap(e.pivotBuf) < needed {
		e.pivotBuf = make([][3]float64, 0, needed*2)
	}
	e.pivotBuf = e.pivotBuf[:1]
	e.pivotBuf[0] = [3]float64{0, 0, 0}

	for k := range bin.Items {
		lo, hi := bin.Box(k)
		e.pivotBuf = append(e.pivotBuf,
			[3]float64{hi[0], lo[1], lo[2]},
			[3]float64{lo[0], hi[1], lo[2]},
			[3]float64{lo[0], lo[1], hi[2]},
		)
	}
	return e.pivotBuf
}
