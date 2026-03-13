package placement

import (
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*PivotEngine)(nil)

// PivotEngine places items using pivot points generated from corners of placed items.
// After placement, fix-point correction pushes items toward the origin to eliminate gaps.
type PivotEngine struct {
	enableStability bool
	supportRatio    float64
	pivotBuf        [][3]float64 // reusable buffer for pivot generation
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

	// Pre-compute dimensions for each rotation (stack-allocated).
	var dims [6][3]float64
	for i, rt := range rotations {
		dims[i] = rotation.DimensionsFor(item, rt)
	}

	bw, bh, bd := bin.Width+epsilon, bin.Height+epsilon, bin.Depth+epsilon

	for _, pivot := range pivots {
		for i, rt := range rotations {
			dim := dims[i]

			// Quick bounds rejection before function call.
			if pivot[0]+dim[0] > bw || pivot[1]+dim[1] > bh || pivot[2]+dim[2] > bd {
				continue
			}

			item.RotationType = rt
			item.Position = pivot

			if !canPlaceDim(bin, item, dim, e.enableStability, e.supportRatio) {
				continue
			}

			// Try fix-point correction for tighter packing.
			savedPos := item.Position
			fixPointDim(bin, item, dim)
			if !canPlaceDim(bin, item, dim, e.enableStability, e.supportRatio) {
				item.Position = savedPos
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
// Uses a reusable buffer to avoid allocations.
func (e *PivotEngine) generatePivots(bin *model.Bin) [][3]float64 {
	needed := 1 + 3*len(bin.Items)
	if cap(e.pivotBuf) < needed {
		e.pivotBuf = make([][3]float64, 0, needed*2)
	}
	e.pivotBuf = e.pivotBuf[:1]
	e.pivotBuf[0] = [3]float64{0, 0, 0}

	for _, placed := range bin.Items {
		dim := placed.Dimension()
		e.pivotBuf = append(e.pivotBuf,
			[3]float64{placed.Position[0] + dim[0], placed.Position[1], placed.Position[2]},
			[3]float64{placed.Position[0], placed.Position[1] + dim[1], placed.Position[2]},
			[3]float64{placed.Position[0], placed.Position[1], placed.Position[2] + dim[2]},
		)
	}
	return e.pivotBuf
}

