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

	for _, pivot := range pivots {
		for _, rt := range rotation.AllowedFor(item) {
			item.RotationType = rt
			item.Position = pivot

			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
				continue
			}

			// Try fix-point correction for tighter packing.
			savedPos := item.Position
			fixPoint(bin, item)
			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
				item.Position = savedPos
			}

			item.Placed = true
			bin.Items = append(bin.Items, item)
			return true
		}
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

// generatePivots returns candidate positions from corners of placed items.
func (e *PivotEngine) generatePivots(bin *model.Bin) [][3]float64 {
	if len(bin.Items) == 0 {
		return [][3]float64{{0, 0, 0}}
	}

	pivots := make([][3]float64, 1, 1+3*len(bin.Items))
	pivots[0] = [3]float64{0, 0, 0}
	for _, placed := range bin.Items {
		dim := placed.Dimension()
		pivots = append(pivots,
			[3]float64{placed.Position[0] + dim[0], placed.Position[1], placed.Position[2]},
			[3]float64{placed.Position[0], placed.Position[1] + dim[1], placed.Position[2]},
			[3]float64{placed.Position[0], placed.Position[1], placed.Position[2] + dim[2]},
		)
	}
	return pivots
}

