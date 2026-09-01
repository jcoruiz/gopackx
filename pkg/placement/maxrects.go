package placement

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
	"github.com/jcoruiz/gopackx/pkg/stability"
)

// Verify interface compliance.
var _ Engine = (*MaxRectsEngine)(nil)

// freeSpace represents a maximal free cuboid within the bin.
type freeSpace struct {
	x, y, z float64
	w, h, d float64
}

// MaxRectsEngine places items using the 3D Maximal Rectangles algorithm
// with gravity correction for tighter packing.
type MaxRectsEngine struct {
	spaces          []freeSpace
	bin             *model.Bin
	enableStability bool
	supportRatio    float64
}

// MaxRectsOption configures the MaxRectsEngine.
type MaxRectsOption func(*MaxRectsEngine)

// WithMaxRectsStability enables stability checking.
func WithMaxRectsStability(ratio float64) MaxRectsOption {
	return func(e *MaxRectsEngine) {
		e.enableStability = true
		e.supportRatio = ratio
	}
}

// NewMaxRectsEngine creates a new MaxRectsEngine.
func NewMaxRectsEngine(opts ...MaxRectsOption) *MaxRectsEngine {
	e := &MaxRectsEngine{}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *MaxRectsEngine) initBin(bin *model.Bin) {
	e.bin = bin
	e.spaces = e.spaces[:0]
	e.spaces = append(e.spaces, freeSpace{
		x: 0, y: 0, z: 0,
		w: bin.Width, h: bin.Height, d: bin.Depth,
	})
	// Rebuild spaces from items already in the bin.
	for _, item := range bin.Items {
		e.splitSpaces(item.Position, item.PlacedDim)
	}
}

// PlaceItem attempts to place an item using maximal rectangles with gravity.
func (e *MaxRectsEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.bin != bin {
		e.initBin(bin)
	}

	// Weight check.
	if item.Weight > bin.MaxWeight-bin.ItemWeight+epsilon {
		return false
	}

	origRT := item.RotationType
	origPos := item.Position

	rots := rotation.AllowedFor(item)

	// Dedup rotations.
	type rotDim struct {
		rt  model.RotationType
		dim [3]float64
	}
	var rds [6]rotDim
	nRot := 0
	for _, rt := range rots {
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

	// Find all valid placements and pick the best.
	bestScore := math.Inf(1)
	found := false
	var bestRT model.RotationType
	var bestDim [3]float64
	var bestPos [3]float64

	for si := range e.spaces {
		s := &e.spaces[si]
		for ri := range nRot {
			dim := rds[ri].dim
			if dim[0] > s.w+epsilon || dim[1] > s.h+epsilon || dim[2] > s.d+epsilon {
				continue
			}

			// Apply gravity: find lowest valid Y at (s.x, s.z).
			pos := [3]float64{s.x, s.y, s.z}
			gravityY := e.findLowestY(bin, pos[0], pos[2], dim)
			if gravityY+dim[1] > bin.Height+epsilon {
				continue
			}
			pos[1] = gravityY

			// BSSF scoring with gravity-corrected Y.
			shortSide := s.w - dim[0]
			dy := s.h - dim[1]
			dz := s.d - dim[2]
			if dy < shortSide {
				shortSide = dy
			}
			if dz < shortSide {
				shortSide = dz
			}
			score := pos[1]*1e10 + shortSide*1e6 + pos[2]*100 + pos[0]

			if score < bestScore {
				bestScore = score
				found = true
				bestRT = rds[ri].rt
				bestDim = dim
				bestPos = pos
			}
		}
	}

	if !found {
		return false
	}

	item.RotationType = bestRT
	item.Position = bestPos

	// Verify no intersection (gravity might create overlap).
	if !canPlaceDim(bin, item, bestDim, false, 0) {
		item.RotationType = origRT
		item.Position = origPos
		return false
	}

	// Fragile check.
	for _, placed := range bin.Items {
		if placed.Fragile {
			pDim := placed.PlacedDim
			pPos := placed.Position
			placedTop := pPos[1] + pDim[1]
			if math.Abs(bestPos[1]-placedTop) <= epsilon {
				ow := overlapLen(bestPos[0], bestDim[0], pPos[0], pDim[0])
				od := overlapLen(bestPos[2], bestDim[2], pPos[2], pDim[2])
				if ow > epsilon && od > epsilon {
					item.RotationType = origRT
					item.Position = origPos
					return false
				}
			}
		}
	}

	// Stability check.
	if e.enableStability {
		if !stability.CheckSupport(item, bin.Items, e.supportRatio) {
			item.RotationType = origRT
			item.Position = origPos
			return false
		}
		bin.Items = append(bin.Items, item)
		loadOK := true
		for _, placed := range bin.Items {
			if placed == item {
				continue
			}
			if !stability.CheckLoadBearing(placed, bin.Items) {
				loadOK = false
				break
			}
		}
		bin.Items = bin.Items[:len(bin.Items)-1]
		if !loadOK {
			item.RotationType = origRT
			item.Position = origPos
			return false
		}
	}

	bin.PlaceItem(item)
	e.splitSpaces(bestPos, bestDim)
	return true
}

// findLowestY finds the lowest valid Y position for an item at (x, z) with given dimensions.
func (e *MaxRectsEngine) findLowestY(bin *model.Bin, x, z float64, dim [3]float64) float64 {
	maxY := 0.0
	for _, placed := range bin.Items {
		pDim := placed.PlacedDim
		pPos := placed.Position
		// Check XZ overlap.
		if x < pPos[0]+pDim[0]-epsilon && pPos[0] < x+dim[0]-epsilon &&
			z < pPos[2]+pDim[2]-epsilon && pPos[2] < z+dim[2]-epsilon {
			top := pPos[1] + pDim[1]
			if top > maxY {
				maxY = top
			}
		}
	}
	return maxY
}

// splitSpaces removes or splits all free spaces that overlap with the placed item.
func (e *MaxRectsEngine) splitSpaces(pos, dim [3]float64) {
	ix0, iy0, iz0 := pos[0], pos[1], pos[2]
	ix1, iy1, iz1 := pos[0]+dim[0], pos[1]+dim[1], pos[2]+dim[2]

	n := len(e.spaces)
	for i := 0; i < n; {
		s := e.spaces[i]
		sx1 := s.x + s.w
		sy1 := s.y + s.h
		sz1 := s.z + s.d

		if ix0 >= sx1-epsilon || s.x >= ix1-epsilon ||
			iy0 >= sy1-epsilon || s.y >= iy1-epsilon ||
			iz0 >= sz1-epsilon || s.z >= iz1-epsilon {
			i++
			continue
		}

		// Remove this space and generate sub-spaces.
		e.spaces[i] = e.spaces[n-1]
		n--
		e.spaces = e.spaces[:n]

		if s.x < ix0-epsilon {
			e.spaces = append(e.spaces, freeSpace{s.x, s.y, s.z, ix0 - s.x, s.h, s.d})
		}
		if sx1 > ix1+epsilon {
			e.spaces = append(e.spaces, freeSpace{ix1, s.y, s.z, sx1 - ix1, s.h, s.d})
		}
		if s.y < iy0-epsilon {
			e.spaces = append(e.spaces, freeSpace{s.x, s.y, s.z, s.w, iy0 - s.y, s.d})
		}
		if sy1 > iy1+epsilon {
			e.spaces = append(e.spaces, freeSpace{s.x, iy1, s.z, s.w, sy1 - iy1, s.d})
		}
		if s.z < iz0-epsilon {
			e.spaces = append(e.spaces, freeSpace{s.x, s.y, s.z, s.w, s.h, iz0 - s.z})
		}
		if sz1 > iz1+epsilon {
			e.spaces = append(e.spaces, freeSpace{s.x, s.y, iz1, s.w, s.h, sz1 - iz1})
		}
	}

	e.pruneContained()
}

// pruneContained removes free spaces fully contained within another.
func (e *MaxRectsEngine) pruneContained() {
	n := len(e.spaces)
	for i := 0; i < n; {
		contained := false
		si := e.spaces[i]
		six1 := si.x + si.w
		siy1 := si.y + si.h
		siz1 := si.z + si.d

		for j := range n {
			if i == j {
				continue
			}
			sj := e.spaces[j]
			if sj.x <= si.x+epsilon && sj.y <= si.y+epsilon && sj.z <= si.z+epsilon &&
				sj.x+sj.w >= six1-epsilon && sj.y+sj.h >= siy1-epsilon && sj.z+sj.d >= siz1-epsilon {
				contained = true
				break
			}
		}

		if contained {
			e.spaces[i] = e.spaces[n-1]
			n--
			e.spaces = e.spaces[:n]
		} else {
			i++
		}
	}
}
