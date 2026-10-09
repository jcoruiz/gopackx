package placement

import (
	"math"
	"sort"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*LAFFEngine)(nil)
var _ Resetter = (*LAFFEngine)(nil)

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
	saved           binStates[[]laffLevel]
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
		e.switchBin(bin)
	}

	origRT := item.RotationType
	origPos := item.Position

	// Try existing levels (most recent first for locality).
	var bl blockers
	for i := len(e.levels) - 1; i >= 0; i-- {
		if e.placeInLevel(bin, item, &e.levels[i], &bl) {
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

// Reset forgets the state kept for every bin. Call it before reusing the
// engine for an unrelated packing run; packer.Packer does it on each Pack.
func (e *LAFFEngine) Reset() {
	e.bin = nil
	e.levels = nil
	e.saved.reset()
}

// switchBin saves the levels of the current bin and restores those of bin,
// rebuilding them only if this engine has not seen bin as it is now.
func (e *LAFFEngine) switchBin(bin *model.Bin) {
	if e.bin != nil {
		e.saved.save(e.bin, e.levels)
	}
	if levels, ok := e.saved.load(bin); ok {
		e.bin, e.levels = bin, levels
		return
	}
	e.initBin(bin)
}

// initBin derives the levels of a bin from its items: one level per height
// position, as tall as the tallest item starting there. Placing an item
// updates the levels the same way (addLevel), so a bin this engine has not
// packed itself gets the same levels as one it has.
func (e *LAFFEngine) initBin(bin *model.Bin) {
	e.bin = bin
	e.levels = nil
	for k := range bin.Items {
		e.addPlaced(bin, k)
	}
}

// addPlaced records the level of item k of the bin.
func (e *LAFFEngine) addPlaced(bin *model.Bin, k int) {
	lo, hi := bin.Box(k)
	e.addLevel(lo[model.HeightAxis], hi[model.HeightAxis]-lo[model.HeightAxis])
}

// addLevel records an item of height h placed at height y.
func (e *LAFFEngine) addLevel(y, h float64) {
	i := sort.Search(len(e.levels), func(i int) bool { return e.levels[i].y >= y-epsilon })
	if i < len(e.levels) && math.Abs(e.levels[i].y-y) <= epsilon {
		e.levels[i].height = math.Max(e.levels[i].height, h)
		return
	}
	e.levels = append(e.levels, laffLevel{})
	copy(e.levels[i+1:], e.levels[i:])
	e.levels[i] = laffLevel{y: y, height: h}
}

// tryNewLevel creates a new level with the item's best rotation (largest base area).
func (e *LAFFEngine) tryNewLevel(bin *model.Bin, item *model.Item) bool {
	// A new level starts on top of everything placed so far.
	newY := 0.0
	for _, lvl := range e.levels {
		newY = math.Max(newY, lvl.y+lvl.height)
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
	e.addPlaced(bin, len(bin.Items)-1)
	return true
}

// placeInLevel tries to place an item within a specific level.
func (e *LAFFEngine) placeInLevel(bin *model.Bin, item *model.Item, lvl *laffLevel, bl *blockers) bool {
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
			if bl.hit(pos, dims) {
				continue
			}
			item.RotationType = rt
			item.Position = pos

			if k := canPlaceDimBlocker(bin, item, dims, e.enableStability, e.supportRatio); k != -1 {
				if k >= 0 {
					bl.add(bin, k)
				}
				continue
			}

			// Apply fix-point correction for tighter packing.
			savedPos := item.Position
			fixPoint(bin, item)
			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
				item.Position = savedPos
			}

			bin.PlaceItem(item)
			e.addPlaced(bin, len(bin.Items)-1)
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

	for k := range bin.Items {
		lo, hi := bin.Box(k)
		// Only consider items in or overlapping this level.
		if lo[1] > lvl.y+lvl.height+epsilon || hi[1] < lvl.y-epsilon {
			continue
		}

		// 2D candidates (on the level floor).
		candidates = append(candidates,
			[3]float64{hi[0], lvl.y, lo[2]},
			[3]float64{lo[0], lvl.y, hi[2]},
		)

		if !e.fast {
			// Full variant: allow stacking within the level.
			stackY := hi[1]
			if stackY < lvl.y+lvl.height-epsilon && stackY >= lvl.y-epsilon {
				candidates = append(candidates,
					[3]float64{lo[0], stackY, lo[2]},
					[3]float64{hi[0], stackY, lo[2]},
					[3]float64{lo[0], stackY, hi[2]},
				)
			}
		}
	}

	return candidates
}
