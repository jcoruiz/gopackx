package placement

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*ExtremePointEngine)(nil)

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
	points          []*ExtremePoint
	bin             *model.Bin
	enableStability bool
	supportRatio    float64
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
	if e.bin != bin {
		e.initBin(bin)
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

			item.RotationType = rt
			item.Position = ep.Pos

			if !canPlace(bin, item, e.enableStability, e.supportRatio) {
				continue
			}

			score := scorePlacement(ep, dims)
			if score < bestScore {
				bestScore = score
				bestPoint = ep
				bestRT = rt
			}
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

	item.Placed = true
	bin.Items = append(bin.Items, item)

	e.onItemPlaced(item)
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

func (e *ExtremePointEngine) initBin(bin *model.Bin) {
	e.bin = bin
	e.points = make([]*ExtremePoint, 0, 8)
	e.points = append(e.points, &ExtremePoint{
		Pos:      [3]float64{0, 0, 0},
		MaxSpace: [3]float64{bin.Width, bin.Height, bin.Depth},
		Support:  3,
	})

	// Rebuild points from items already in the bin.
	for _, item := range bin.Items {
		e.onItemPlaced(item)
	}
}

func (e *ExtremePointEngine) onItemPlaced(item *model.Item) {
	dim := item.Dimension()

	e.removePointsInside(item, dim)
	e.generatePoints(item, dim)
	e.recalculateMaxSpace()
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
			continue // strictly inside, remove
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
	for _, placed := range e.bin.Items {
		if placed == item {
			continue
		}
		pd := placed.Dimension()
		pp := placed.Position

		// Existing item's right face cuts through new item's X range.
		rightX := pp[0] + pd[0]
		if rightX > px+epsilon && rightX < px+dim[0]-epsilon {
			candidates = append(candidates,
				[3]float64{rightX, py + dim[1], pz},
				[3]float64{rightX, py, pz + dim[2]},
			)
		}

		// Existing item's top face cuts through new item's Y range.
		topY := pp[1] + pd[1]
		if topY > py+epsilon && topY < py+dim[1]-epsilon {
			candidates = append(candidates,
				[3]float64{px + dim[0], topY, pz},
				[3]float64{px, topY, pz + dim[2]},
			)
		}

		// Existing item's back face cuts through new item's Z range.
		backZ := pp[2] + pd[2]
		if backZ > pz+epsilon && backZ < pz+dim[2]-epsilon {
			candidates = append(candidates,
				[3]float64{px + dim[0], py, backZ},
				[3]float64{px, py + dim[1], backZ},
			)
		}

		// New item's faces cut through existing item's ranges (reverse direction).
		newRight := px + dim[0]
		if newRight > pp[0]+epsilon && newRight < pp[0]+pd[0]-epsilon {
			candidates = append(candidates,
				[3]float64{newRight, pp[1] + pd[1], pp[2]},
				[3]float64{newRight, pp[1], pp[2] + pd[2]},
			)
		}

		newTop := py + dim[1]
		if newTop > pp[1]+epsilon && newTop < pp[1]+pd[1]-epsilon {
			candidates = append(candidates,
				[3]float64{pp[0] + pd[0], newTop, pp[2]},
				[3]float64{pp[0], newTop, pp[2] + pd[2]},
			)
		}

		newBack := pz + dim[2]
		if newBack > pp[2]+epsilon && newBack < pp[2]+pd[2]-epsilon {
			candidates = append(candidates,
				[3]float64{pp[0] + pd[0], pp[1], newBack},
				[3]float64{pp[0], pp[1] + pd[1], newBack},
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

		if e.isInsideAnyItem(pos) {
			continue
		}

		if e.isDuplicate(pos) {
			continue
		}

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
	for _, item := range e.bin.Items {
		dim := item.Dimension()
		itemTop := item.Position[1] + dim[1]

		if pos[0] >= item.Position[0]-epsilon && pos[0] < item.Position[0]+dim[0]+epsilon &&
			pos[2] >= item.Position[2]-epsilon && pos[2] < item.Position[2]+dim[2]+epsilon &&
			itemTop <= pos[1]+epsilon && itemTop > bestY {
			bestY = itemTop
		}
	}

	pos[1] = bestY
	return pos
}

func (e *ExtremePointEngine) isInsideAnyItem(pos [3]float64) bool {
	for _, item := range e.bin.Items {
		dim := item.Dimension()
		if pos[0] > item.Position[0]+epsilon &&
			pos[0] < item.Position[0]+dim[0]-epsilon &&
			pos[1] > item.Position[1]+epsilon &&
			pos[1] < item.Position[1]+dim[1]-epsilon &&
			pos[2] > item.Position[2]+epsilon &&
			pos[2] < item.Position[2]+dim[2]-epsilon {
			return true
		}
	}
	return false
}

func (e *ExtremePointEngine) isDuplicate(pos [3]float64) bool {
	for _, ep := range e.points {
		if math.Abs(ep.Pos[0]-pos[0]) < epsilon &&
			math.Abs(ep.Pos[1]-pos[1]) < epsilon &&
			math.Abs(ep.Pos[2]-pos[2]) < epsilon {
			return true
		}
	}
	return false
}

// countSupport counts how many planes (XY=floor, XZ=side wall, YZ=front wall)
// support the given position.
func (e *ExtremePointEngine) countSupport(pos [3]float64) int {
	support := 0

	// XY plane support (floor or item top surface).
	if pos[1] < epsilon {
		support++
	} else {
		for _, item := range e.bin.Items {
			dim := item.Dimension()
			itemTop := item.Position[1] + dim[1]
			if math.Abs(pos[1]-itemTop) < epsilon &&
				pos[0] >= item.Position[0]-epsilon && pos[0] < item.Position[0]+dim[0]+epsilon &&
				pos[2] >= item.Position[2]-epsilon && pos[2] < item.Position[2]+dim[2]+epsilon {
				support++
				break
			}
		}
	}

	// YZ plane support (left wall or item right face).
	if pos[0] < epsilon {
		support++
	} else {
		for _, item := range e.bin.Items {
			dim := item.Dimension()
			itemRight := item.Position[0] + dim[0]
			if math.Abs(pos[0]-itemRight) < epsilon &&
				pos[1] >= item.Position[1]-epsilon && pos[1] < item.Position[1]+dim[1]+epsilon &&
				pos[2] >= item.Position[2]-epsilon && pos[2] < item.Position[2]+dim[2]+epsilon {
				support++
				break
			}
		}
	}

	// XZ plane support (front wall or item back face).
	if pos[2] < epsilon {
		support++
	} else {
		for _, item := range e.bin.Items {
			dim := item.Dimension()
			itemBack := item.Position[2] + dim[2]
			if math.Abs(pos[2]-itemBack) < epsilon &&
				pos[0] >= item.Position[0]-epsilon && pos[0] < item.Position[0]+dim[0]+epsilon &&
				pos[1] >= item.Position[1]-epsilon && pos[1] < item.Position[1]+dim[1]+epsilon {
				support++
				break
			}
		}
	}

	return support
}

// recalculateMaxSpace updates max available space for all points.
func (e *ExtremePointEngine) recalculateMaxSpace() {
	for _, ep := range e.points {
		e.calculateMaxSpace(ep)
	}

	// Remove points with zero max space on any axis.
	n := 0
	for _, ep := range e.points {
		if ep.MaxSpace[0] > epsilon && ep.MaxSpace[1] > epsilon && ep.MaxSpace[2] > epsilon {
			e.points[n] = ep
			n++
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

	for _, item := range e.bin.Items {
		dim := item.Dimension()
		ip := item.Position

		// Width: item to the right, point within item's Y-Z cross-section.
		if ip[0] > ep.Pos[0]-epsilon &&
			ep.Pos[1] >= ip[1]-epsilon && ep.Pos[1] < ip[1]+dim[1]-epsilon &&
			ep.Pos[2] >= ip[2]-epsilon && ep.Pos[2] < ip[2]+dim[2]-epsilon {
			gap := ip[0] - ep.Pos[0]
			if gap >= -epsilon && gap < ep.MaxSpace[0] {
				ep.MaxSpace[0] = math.Max(0, gap)
			}
		}

		// Height: item above, point within item's X-Z cross-section.
		if ip[1] > ep.Pos[1]-epsilon &&
			ep.Pos[0] >= ip[0]-epsilon && ep.Pos[0] < ip[0]+dim[0]-epsilon &&
			ep.Pos[2] >= ip[2]-epsilon && ep.Pos[2] < ip[2]+dim[2]-epsilon {
			gap := ip[1] - ep.Pos[1]
			if gap >= -epsilon && gap < ep.MaxSpace[1] {
				ep.MaxSpace[1] = math.Max(0, gap)
			}
		}

		// Depth: item behind, point within item's X-Y cross-section.
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
