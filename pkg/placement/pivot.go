package placement

import (
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*PivotEngine)(nil)

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
		for j := 0; j < nRot; j++ {
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
	data := bin.AABBData
	const maxBlockers = 4
	var blockers [maxBlockers][6]float64
	nBlockers := 0
	writeIdx := 0

	for _, pivot := range pivots {
		for ri := 0; ri < nRot; ri++ {
			dim := rds[ri].dim

			px1 := pivot[0] + dim[0]
			py1 := pivot[1] + dim[1]
			pz1 := pivot[2] + dim[2]
			if px1 > bw || py1 > bh || pz1 > bd {
				continue
			}

			// Conflict-driven pre-rejection from stack-cached blockers.
			blocked := false
			for bi := 0; bi < nBlockers; bi++ {
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
				off := blocker * 6
				blockers[writeIdx] = [6]float64{data[off], data[off+1], data[off+2], data[off+3], data[off+4], data[off+5]}
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
			return true
		}
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

// generatePivots returns candidate positions from corners of placed items.
func (e *PivotEngine) generatePivots(bin *model.Bin) [][3]float64 {
	needed := 1 + 3*len(bin.Items)
	if cap(e.pivotBuf) < needed {
		e.pivotBuf = make([][3]float64, 0, needed*2)
	}
	e.pivotBuf = e.pivotBuf[:1]
	e.pivotBuf[0] = [3]float64{0, 0, 0}

	for _, placed := range bin.Items {
		d := placed.PlacedDim
		e.pivotBuf = append(e.pivotBuf,
			[3]float64{placed.Position[0] + d[0], placed.Position[1], placed.Position[2]},
			[3]float64{placed.Position[0], placed.Position[1] + d[1], placed.Position[2]},
			[3]float64{placed.Position[0], placed.Position[1], placed.Position[2] + d[2]},
		)
	}
	return e.pivotBuf
}
