package placement

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*ExtremePointEngine)(nil)
var _ Resetter = (*ExtremePointEngine)(nil)

// ExtremePoint represents a candidate position with metadata about available space
// and supporting surfaces.
type ExtremePoint struct {
	Pos      [3]float64 // candidate position
	MaxSpace [3]float64 // max item dimensions available from this point
	Support  int        // number of supporting planes (0-3)
}

// ExtremePointEngine places items using the extreme points algorithm.
// It maintains a list of candidate positions with space and support metadata,
// enabling fast rejection and better placement scoring than simple pivot points.
type ExtremePointEngine struct {
	points []*ExtremePoint
	keys   map[[3]int64]struct{} // positions of points, to skip duplicates
	bin    *model.Bin
	// items are the bin's items the points are computed against: all of
	// them while packing, the ones placed so far while rebuilding. boxes
	// holds their corners (x0, y0, z0, x1, y1, z1 each), a compact copy
	// for the hot loops.
	items           []*model.Item
	boxes           []float64
	rev             uint64 // bin revision the state matches
	saved           binStates[epState]
	enableStability bool
	supportRatio    float64
}

// epState is the per-bin state an ExtremePointEngine remembers.
type epState struct {
	points []*ExtremePoint
	keys   map[[3]int64]struct{}
	boxes  []float64
}

// ExtremePointOption configures the ExtremePointEngine.
type ExtremePointOption func(*ExtremePointEngine)

// WithEPStability enables stability checking with the given support ratio.
func WithEPStability(ratio float64) ExtremePointOption {
	return func(e *ExtremePointEngine) {
		e.enableStability = true
		e.supportRatio = ratio
	}
}

// NewExtremePointEngine creates a new ExtremePointEngine.
func NewExtremePointEngine(opts ...ExtremePointOption) *ExtremePointEngine {
	e := &ExtremePointEngine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// PlaceItem attempts to place an item using extreme points.
// It evaluates all candidate points and rotations, selecting the best placement
// based on support, position, and fit quality.
func (e *ExtremePointEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.bin != bin || e.rev != bin.Revision() {
		e.switchBin(bin)
	}

	origRT := item.RotationType
	origPos := item.Position

	var bestPoint *ExtremePoint
	var bestRT model.RotationType
	bestScore := math.Inf(1)

	for _, rt := range rotation.AllowedFor(item) {
		dims := rotation.DimensionsFor(item, rt)

		for _, ep := range e.points {
			// Quick rejection: item doesn't fit in available space.
			if dims[0] > ep.MaxSpace[0]+epsilon ||
				dims[1] > ep.MaxSpace[1]+epsilon ||
				dims[2] > ep.MaxSpace[2]+epsilon {
				continue
			}

			// Scoring is cheap and does not depend on validity, so the full
			// placement check only runs for points that could beat the best.
			score := scorePlacement(ep, dims)
			if score >= bestScore {
				continue
			}

			item.RotationType = rt
			item.Position = ep.Pos

			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
				continue
			}

			bestScore = score
			bestPoint = ep
			bestRT = rt
		}
	}

	if bestPoint == nil {
		item.RotationType = origRT
		item.Position = origPos
		return false
	}

	item.RotationType = bestRT
	item.Position = bestPoint.Pos

	// Apply fix-point correction for tighter packing.
	savedPos := item.Position
	fixPoint(bin, item)
	if !canPlace(bin, item, e.enableStability, e.supportRatio) {
		item.Position = savedPos
	}

	bin.PlaceItem(item)

	e.items = bin.Items
	e.onItemPlaced(item)
	e.rev = bin.Revision()
	return true
}

// scorePlacement scores a placement candidate (lower is better).
// Priorities: more support > lower position > closer to origin > tighter fit.
func scorePlacement(ep *ExtremePoint, dims [3]float64) float64 {
	supportPenalty := float64(3-ep.Support) * 1e6
	positionScore := ep.Pos[1]*1e4 + ep.Pos[2]*1e2 + ep.Pos[0]
	wasteScore := (ep.MaxSpace[0] - dims[0]) + (ep.MaxSpace[1] - dims[1]) + (ep.MaxSpace[2] - dims[2])
	return supportPenalty + positionScore + wasteScore
}

// Reset forgets the state kept for every bin. Call it before reusing the
// engine for an unrelated packing run; packer.Packer does it on each Pack.
func (e *ExtremePointEngine) Reset() {
	e.bin = nil
	e.points, e.keys, e.items, e.boxes = nil, nil, nil, nil
	e.saved.reset()
}

// switchBin saves the points of the current bin and restores those of bin,
// rebuilding them only if this engine has not seen bin in its current state.
func (e *ExtremePointEngine) switchBin(bin *model.Bin) {
	// A state for a bin that changed behind the engine's back is dropped.
	if e.bin != nil && e.bin != bin {
		e.saved.save(e.bin, epState{e.points, e.keys, e.boxes}, e.rev)
	}
	if st, ok := e.saved.load(bin); ok {
		e.bin, e.items, e.points, e.keys, e.boxes = bin, bin.Items, st.points, st.keys, st.boxes
		e.rev = bin.Revision()
		return
	}
	e.initBin(bin)
}

func (e *ExtremePointEngine) initBin(bin *model.Bin) {
	e.bin = bin
	origin := &ExtremePoint{
		Pos:      [3]float64{0, 0, 0},
		MaxSpace: [3]float64{bin.Width, bin.Height, bin.Depth},
		Support:  3,
	}
	e.points = []*ExtremePoint{origin}
	e.keys = map[[3]int64]struct{}{pointKey(origin.Pos): {}}
	e.boxes = nil

	// Rebuild points from items already in the bin, replaying them in order
	// so the result is the same as if this engine had placed them.
	for k, item := range bin.Items {
		e.items = bin.Items[:k+1]
		e.onItemPlaced(item)
	}
	e.items = bin.Items
	e.rev = bin.Revision()
}

func (e *ExtremePointEngine) onItemPlaced(item *model.Item) {
	dim := item.Dimension()
	lo, hi := e.bin.Box(len(e.items) - 1)
	e.boxes = append(e.boxes, lo[0], lo[1], lo[2], hi[0], hi[1], hi[2])

	e.removePointsInside(item, dim)

	// Incrementally update MaxSpace for existing points against only the new item.
	// MaxSpace can only decrease when a new item is added (Crainic et al. 2008, Algorithm 2).
	e.updateMaxSpaceIncremental(item, dim)

	// Generate new candidate points from the placed item.
	oldCount := len(e.points)
	e.generatePoints(item, dim)

	// New points need full MaxSpace calculation against all items.
	for i := oldCount; i < len(e.points); i++ {
		e.calculateMaxSpace(e.points[i])
	}

	// Remove points with zero max space on any axis.
	e.removeZeroSpacePoints()
}

// removePointsInside removes candidate points that fall inside the placed item's volume.
func (e *ExtremePointEngine) removePointsInside(item *model.Item, dim [3]float64) {
	n := 0
	for _, ep := range e.points {
		if ep.Pos[0] > item.Position[0]+epsilon &&
			ep.Pos[0] < item.Position[0]+dim[0]-epsilon &&
			ep.Pos[1] > item.Position[1]+epsilon &&
			ep.Pos[1] < item.Position[1]+dim[1]-epsilon &&
			ep.Pos[2] > item.Position[2]+epsilon &&
			ep.Pos[2] < item.Position[2]+dim[2]-epsilon {
			delete(e.keys, pointKey(ep.Pos)) // strictly inside, remove
			continue
		}
		e.points[n] = ep
		n++
	}
	e.points = e.points[:n]
}

// generatePoints creates new extreme points from a placed item's faces
// and from intersections with existing items.
func (e *ExtremePointEngine) generatePoints(item *model.Item, dim [3]float64) {
	px, py, pz := item.Position[0], item.Position[1], item.Position[2]

	// Basic corner points (3 far corners of the placed item).
	candidates := make([][3]float64, 0, 16)
	candidates = append(candidates,
		[3]float64{px + dim[0], py, pz},
		[3]float64{px, py + dim[1], pz},
		[3]float64{px, py, pz + dim[2]},
	)

	// Interaction points: where existing items' faces intersect with the new item.
	// Every placed item but the new one, which is last.
	for o := e.boxes[:len(e.boxes)-6]; len(o) >= 6; o = o[6:] {
		pp, phi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}

		// Existing item's right face cuts through new item's X range.
		rightX := phi[0]
		if rightX > px+epsilon && rightX < px+dim[0]-epsilon {
			candidates = append(candidates,
				[3]float64{rightX, py + dim[1], pz},
				[3]float64{rightX, py, pz + dim[2]},
			)
		}

		// Existing item's top face cuts through new item's Y range.
		topY := phi[1]
		if topY > py+epsilon && topY < py+dim[1]-epsilon {
			candidates = append(candidates,
				[3]float64{px + dim[0], topY, pz},
				[3]float64{px, topY, pz + dim[2]},
			)
		}

		// Existing item's back face cuts through new item's Z range.
		backZ := phi[2]
		if backZ > pz+epsilon && backZ < pz+dim[2]-epsilon {
			candidates = append(candidates,
				[3]float64{px + dim[0], py, backZ},
				[3]float64{px, py + dim[1], backZ},
			)
		}

		// New item's faces cut through existing item's ranges (reverse direction).
		newRight := px + dim[0]
		if newRight > pp[0]+epsilon && newRight < phi[0]-epsilon {
			candidates = append(candidates,
				[3]float64{newRight, phi[1], pp[2]},
				[3]float64{newRight, pp[1], phi[2]},
			)
		}

		newTop := py + dim[1]
		if newTop > pp[1]+epsilon && newTop < phi[1]-epsilon {
			candidates = append(candidates,
				[3]float64{phi[0], newTop, pp[2]},
				[3]float64{pp[0], newTop, phi[2]},
			)
		}

		newBack := pz + dim[2]
		if newBack > pp[2]+epsilon && newBack < phi[2]-epsilon {
			candidates = append(candidates,
				[3]float64{phi[0], pp[1], newBack},
				[3]float64{pp[0], phi[1], newBack},
			)
		}
	}

	for i := range candidates {
		pos := e.projectDown(candidates[i])

		// Must be within bin bounds.
		if pos[0] < -epsilon || pos[1] < -epsilon || pos[2] < -epsilon ||
			pos[0] > e.bin.Width+epsilon || pos[1] > e.bin.Height+epsilon || pos[2] > e.bin.Depth+epsilon {
			continue
		}

		key := pointKey(pos)
		if _, dup := e.keys[key]; dup {
			continue
		}

		if e.isInsideAnyItem(pos) {
			continue
		}

		e.keys[key] = struct{}{}
		e.points = append(e.points, &ExtremePoint{
			Pos:     pos,
			Support: e.countSupport(pos),
		})
	}
}

// projectDown moves a point down to the nearest supporting surface (gravity).
func (e *ExtremePointEngine) projectDown(pos [3]float64) [3]float64 {
	if pos[1] < epsilon {
		return pos
	}

	bestY := 0.0
	for o := e.boxes; len(o) >= 6; o = o[6:] {
		lo, hi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}
		itemTop := hi[1]

		if pos[0] >= lo[0]-epsilon && pos[0] < hi[0]+epsilon &&
			pos[2] >= lo[2]-epsilon && pos[2] < hi[2]+epsilon &&
			itemTop <= pos[1]+epsilon && itemTop > bestY {
			bestY = itemTop
		}
	}

	pos[1] = bestY
	return pos
}

func (e *ExtremePointEngine) isInsideAnyItem(pos [3]float64) bool {
	for o := e.boxes; len(o) >= 6; o = o[6:] {
		lo, hi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}
		if pos[0] > lo[0]+epsilon &&
			pos[0] < hi[0]-epsilon &&
			pos[1] > lo[1]+epsilon &&
			pos[1] < hi[1]-epsilon &&
			pos[2] > lo[2]+epsilon &&
			pos[2] < hi[2]-epsilon {
			return true
		}
	}
	return false
}

// pointKey snaps a position to the epsilon grid that tells points apart.
func pointKey(p [3]float64) [3]int64 {
	return [3]int64{
		int64(math.Round(p[0] / epsilon)),
		int64(math.Round(p[1] / epsilon)),
		int64(math.Round(p[2] / epsilon)),
	}
}

// countSupport counts how many planes (XY=floor, XZ=side wall, YZ=front wall)
// support the given position.
func (e *ExtremePointEngine) countSupport(pos [3]float64) int {
	support := 0

	// XY plane support (floor or item top surface).
	if pos[1] < epsilon {
		support++
	} else {
		for o := e.boxes; len(o) >= 6; o = o[6:] {
			lo, hi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}
			itemTop := hi[1]
			if math.Abs(pos[1]-itemTop) < epsilon &&
				pos[0] >= lo[0]-epsilon && pos[0] < hi[0]+epsilon &&
				pos[2] >= lo[2]-epsilon && pos[2] < hi[2]+epsilon {
				support++
				break
			}
		}
	}

	// YZ plane support (left wall or item right face).
	if pos[0] < epsilon {
		support++
	} else {
		for o := e.boxes; len(o) >= 6; o = o[6:] {
			lo, hi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}
			itemRight := hi[0]
			if math.Abs(pos[0]-itemRight) < epsilon &&
				pos[1] >= lo[1]-epsilon && pos[1] < hi[1]+epsilon &&
				pos[2] >= lo[2]-epsilon && pos[2] < hi[2]+epsilon {
				support++
				break
			}
		}
	}

	// XZ plane support (front wall or item back face).
	if pos[2] < epsilon {
		support++
	} else {
		for o := e.boxes; len(o) >= 6; o = o[6:] {
			lo, hi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}
			itemBack := hi[2]
			if math.Abs(pos[2]-itemBack) < epsilon &&
				pos[0] >= lo[0]-epsilon && pos[0] < hi[0]+epsilon &&
				pos[1] >= lo[1]-epsilon && pos[1] < hi[1]+epsilon {
				support++
				break
			}
		}
	}

	return support
}

// updateMaxSpaceIncremental reduces MaxSpace of existing points based on a newly placed item.
// This is the core optimization: instead of recalculating all points against all items O(P*n),
// we only check the new item against all points O(P), since MaxSpace can only shrink.
func (e *ExtremePointEngine) updateMaxSpaceIncremental(item *model.Item, dim [3]float64) {
	ip := item.Position

	for _, ep := range e.points {
		// Width axis: new item blocks to the right.
		if ip[0] > ep.Pos[0]-epsilon &&
			ep.Pos[1] >= ip[1]-epsilon && ep.Pos[1] < ip[1]+dim[1]-epsilon &&
			ep.Pos[2] >= ip[2]-epsilon && ep.Pos[2] < ip[2]+dim[2]-epsilon {
			gap := ip[0] - ep.Pos[0]
			if gap >= -epsilon && gap < ep.MaxSpace[0] {
				ep.MaxSpace[0] = math.Max(0, gap)
			}
		}

		// Height axis: new item blocks above.
		if ip[1] > ep.Pos[1]-epsilon &&
			ep.Pos[0] >= ip[0]-epsilon && ep.Pos[0] < ip[0]+dim[0]-epsilon &&
			ep.Pos[2] >= ip[2]-epsilon && ep.Pos[2] < ip[2]+dim[2]-epsilon {
			gap := ip[1] - ep.Pos[1]
			if gap >= -epsilon && gap < ep.MaxSpace[1] {
				ep.MaxSpace[1] = math.Max(0, gap)
			}
		}

		// Depth axis: new item blocks behind.
		if ip[2] > ep.Pos[2]-epsilon &&
			ep.Pos[0] >= ip[0]-epsilon && ep.Pos[0] < ip[0]+dim[0]-epsilon &&
			ep.Pos[1] >= ip[1]-epsilon && ep.Pos[1] < ip[1]+dim[1]-epsilon {
			gap := ip[2] - ep.Pos[2]
			if gap >= -epsilon && gap < ep.MaxSpace[2] {
				ep.MaxSpace[2] = math.Max(0, gap)
			}
		}
	}
}

// removeZeroSpacePoints removes points with zero max space on any axis.
func (e *ExtremePointEngine) removeZeroSpacePoints() {
	n := 0
	for _, ep := range e.points {
		if ep.MaxSpace[0] > epsilon && ep.MaxSpace[1] > epsilon && ep.MaxSpace[2] > epsilon {
			e.points[n] = ep
			n++
		} else {
			delete(e.keys, pointKey(ep.Pos))
		}
	}
	e.points = e.points[:n]
}

// calculateMaxSpace computes the maximum item dimensions that can fit at this point.
// For each axis, finds the nearest item or wall blocking that direction.
func (e *ExtremePointEngine) calculateMaxSpace(ep *ExtremePoint) {
	ep.MaxSpace = [3]float64{
		e.bin.Width - ep.Pos[0],
		e.bin.Height - ep.Pos[1],
		e.bin.Depth - ep.Pos[2],
	}

	for o := e.boxes; len(o) >= 6; o = o[6:] {
		lo, hi := [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}

		// Width: item to the right, point within item's Y-Z cross-section.
		if lo[0] > ep.Pos[0]-epsilon &&
			ep.Pos[1] >= lo[1]-epsilon && ep.Pos[1] < hi[1]-epsilon &&
			ep.Pos[2] >= lo[2]-epsilon && ep.Pos[2] < hi[2]-epsilon {
			gap := lo[0] - ep.Pos[0]
			if gap >= -epsilon && gap < ep.MaxSpace[0] {
				ep.MaxSpace[0] = math.Max(0, gap)
			}
		}

		// Height: item above, point within item's X-Z cross-section.
		if lo[1] > ep.Pos[1]-epsilon &&
			ep.Pos[0] >= lo[0]-epsilon && ep.Pos[0] < hi[0]-epsilon &&
			ep.Pos[2] >= lo[2]-epsilon && ep.Pos[2] < hi[2]-epsilon {
			gap := lo[1] - ep.Pos[1]
			if gap >= -epsilon && gap < ep.MaxSpace[1] {
				ep.MaxSpace[1] = math.Max(0, gap)
			}
		}

		// Depth: item behind, point within item's X-Y cross-section.
		if lo[2] > ep.Pos[2]-epsilon &&
			ep.Pos[0] >= lo[0]-epsilon && ep.Pos[0] < hi[0]-epsilon &&
			ep.Pos[1] >= lo[1]-epsilon && ep.Pos[1] < hi[1]-epsilon {
			gap := lo[2] - ep.Pos[2]
			if gap >= -epsilon && gap < ep.MaxSpace[2] {
				ep.MaxSpace[2] = math.Max(0, gap)
			}
		}
	}
}
