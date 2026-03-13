package placement

import (
	"sort"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*LAFFEngine)(nil)

type laffLevel struct {
	y      float64
	height float64
}

// LAFFEngine implements Largest Area Fit First packing.
// It divides the bin into horizontal levels where the first item of each level
// defines its height. Items are packed within levels, then levels are stacked.
type LAFFEngine struct {
	bin             *model.Bin
	levels          []laffLevel
	enableStability bool
	supportRatio    float64
	fast            bool // fast variant: 2D-only placement within levels
}

// LAFFOption configures the LAFFEngine.
type LAFFOption func(*LAFFEngine)

// WithLAFFStability enables stability checking with the given support ratio.
func WithLAFFStability(ratio float64) LAFFOption {
	return func(e *LAFFEngine) {
		e.enableStability = true
		e.supportRatio = ratio
	}
}

// LAFFFast enables the fast variant (2D-only placement within levels).
func LAFFFast() LAFFOption {
	return func(e *LAFFEngine) { e.fast = true }
}

// NewLAFFEngine creates a new LAFFEngine.
func NewLAFFEngine(opts ...LAFFOption) *LAFFEngine {
	e := &LAFFEngine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// PlaceItem attempts to place an item within existing levels or by creating a new one.
func (e *LAFFEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.bin != bin {
		e.initBin(bin)
	}

	origRT := item.RotationType
	origPos := item.Position

	// Try existing levels (most recent first for locality).
	for i := len(e.levels) - 1; i >= 0; i-- {
		if e.placeInLevel(bin, item, &e.levels[i]) {
			return true
		}
	}

	// Create a new level.
	if e.tryNewLevel(bin, item) {
		return true
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

func (e *LAFFEngine) initBin(bin *model.Bin) {
	e.bin = bin
	e.levels = nil

	if len(bin.Items) == 0 {
		return
	}

	// Rebuild levels from existing items grouped by Y position.
	seen := make(map[float64]float64) // y → max height at that y
	for _, item := range bin.Items {
		y := item.Position[model.HeightAxis]
		dim := item.Dimension()
		h := dim[model.HeightAxis]
		if cur, ok := seen[y]; !ok || h > cur {
			seen[y] = h
		}
	}
	e.levels = make([]laffLevel, 0, len(seen))
	for y, h := range seen {
		e.levels = append(e.levels, laffLevel{y: y, height: h})
	}
	sort.Slice(e.levels, func(i, j int) bool {
		return e.levels[i].y < e.levels[j].y
	})
}

// tryNewLevel creates a new level with the item's best rotation (largest base area).
func (e *LAFFEngine) tryNewLevel(bin *model.Bin, item *model.Item) bool {
	newY := 0.0
	if len(e.levels) > 0 {
		last := e.levels[len(e.levels)-1]
		newY = last.y + last.height
	}

	// Find rotation with largest base area that fits in remaining height.
	bestRT := model.RotationType(-1)
	bestArea := -1.0

	for _, rt := range rotation.AllowedFor(item) {
		dims := rotation.DimensionsFor(item, rt)
		if newY+dims[1] > bin.Height+epsilon {
			continue
		}
		if dims[0] > bin.Width+epsilon || dims[2] > bin.Depth+epsilon {
			continue
		}
		area := dims[0] * dims[2]
		if area > bestArea {
			bestArea = area
			bestRT = rt
		}
	}

	if bestRT < 0 {
		return false
	}

	dims := rotation.DimensionsFor(item, bestRT)
	lvl := laffLevel{y: newY, height: dims[1]}

	item.RotationType = bestRT
	item.Position = [3]float64{0, newY, 0}

	if !canPlace(bin, item, e.enableStability, e.supportRatio) {
		return false
	}

	// Apply fix-point correction for tighter packing.
	savedPos := item.Position
	fixPoint(bin, item)
	if !canPlace(bin, item, e.enableStability, e.supportRatio) {
		item.Position = savedPos
	}

	bin.PlaceItem(item)
	e.levels = append(e.levels, lvl)
	return true
}

// placeInLevel tries to place an item within a specific level.
func (e *LAFFEngine) placeInLevel(bin *model.Bin, item *model.Item, lvl *laffLevel) bool {
	origRT := item.RotationType
	origPos := item.Position

	candidates := e.levelCandidates(bin, lvl)

	for _, rt := range rotation.AllowedFor(item) {
		dims := rotation.DimensionsFor(item, rt)

		// Must fit within level height.
		if dims[1] > lvl.height+epsilon {
			continue
		}

		for _, pos := range candidates {
			item.RotationType = rt
			item.Position = pos

			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
				continue
			}

			// Apply fix-point correction for tighter packing.
			savedPos := item.Position
			fixPoint(bin, item)
			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
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

// levelCandidates generates candidate positions within a level.
func (e *LAFFEngine) levelCandidates(bin *model.Bin, lvl *laffLevel) [][3]float64 {
	candidates := make([][3]float64, 0, 1+3*len(bin.Items))
	candidates = append(candidates, [3]float64{0, lvl.y, 0})

	for _, placed := range bin.Items {
		py := placed.Position[model.HeightAxis]
		// Only consider items in or overlapping this level.
		if py > lvl.y+lvl.height+epsilon || py+placed.Dimension()[model.HeightAxis] < lvl.y-epsilon {
			continue
		}

		dim := placed.Dimension()
		pp := placed.Position

		// 2D candidates (on the level floor).
		candidates = append(candidates,
			[3]float64{pp[0] + dim[0], lvl.y, pp[2]},
			[3]float64{pp[0], lvl.y, pp[2] + dim[2]},
		)

		if !e.fast {
			// Full variant: allow stacking within the level.
			stackY := pp[1] + dim[1]
			if stackY < lvl.y+lvl.height-epsilon && stackY >= lvl.y-epsilon {
				candidates = append(candidates,
					[3]float64{pp[0], stackY, pp[2]},
					[3]float64{pp[0] + dim[0], stackY, pp[2]},
					[3]float64{pp[0], stackY, pp[2] + dim[2]},
				)
			}
		}
	}

	return candidates
}
