package placement

import (
	"sort"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/rotation"
)

// Verify interface compliance.
var _ Engine = (*MaxRectsEngine)(nil)
var _ Resetter = (*MaxRectsEngine)(nil)

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
	saved           binStates[[]freeSpace]
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

// Reset forgets the state kept for every bin. Call it before reusing the
// engine for an unrelated packing run; packer.Packer does it on each Pack.
func (e *MaxRectsEngine) Reset() {
	e.bin = nil
	e.spaces = nil
	e.saved.reset()
}

// switchBin saves the free spaces of the current bin and restores those of
// bin, rebuilding them only if this engine has not seen bin as it is now.
func (e *MaxRectsEngine) switchBin(bin *model.Bin) {
	if e.bin != nil {
		e.saved.save(e.bin, e.spaces)
	}
	if spaces, ok := e.saved.load(bin); ok {
		e.bin, e.spaces = bin, spaces
		return
	}
	e.initBin(bin)
}

func (e *MaxRectsEngine) initBin(bin *model.Bin) {
	e.bin = bin
	// A new slice: the previous one may be saved for another bin.
	e.spaces = make([]freeSpace, 0, 16)
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
// Candidate spots are tried from the best score down until one passes the
// full placement check (overlap, fragile items, load limits, stability).
func (e *MaxRectsEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.bin != bin {
		e.switchBin(bin)
	}

	// Weight check.
	if !bin.CanCarry(item.Weight) {
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

	type candidate struct {
		score float64
		rt    model.RotationType
		dim   [3]float64
		pos   [3]float64
	}
	var candidates []candidate

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
			candidates = append(candidates, candidate{score, rds[ri].rt, dim, pos})
		}
	}

	// Stable sort keeps the space order for equal scores, as before.
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score < candidates[j].score })

	for _, c := range candidates {
		item.RotationType = c.rt
		item.Position = c.pos
		// Gravity can move a spot onto other items; the full check covers it.
		if !canPlaceDim(bin, item, c.dim, e.enableStability, e.supportRatio) {
			continue
		}
		bin.PlaceItem(item)
		e.splitSpaces(c.pos, c.dim)
		return true
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

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

	// Sub-spaces are collected apart and added after the loop: appending them
	// to e.spaces while removing split spaces from it would let a later
	// removal truncate the sub-spaces of an earlier split.
	var split []freeSpace
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

		if s.x < ix0-epsilon {
			split = append(split, freeSpace{s.x, s.y, s.z, ix0 - s.x, s.h, s.d})
		}
		if sx1 > ix1+epsilon {
			split = append(split, freeSpace{ix1, s.y, s.z, sx1 - ix1, s.h, s.d})
		}
		if s.y < iy0-epsilon {
			split = append(split, freeSpace{s.x, s.y, s.z, s.w, iy0 - s.y, s.d})
		}
		if sy1 > iy1+epsilon {
			split = append(split, freeSpace{s.x, iy1, s.z, s.w, sy1 - iy1, s.d})
		}
		if s.z < iz0-epsilon {
			split = append(split, freeSpace{s.x, s.y, s.z, s.w, s.h, iz0 - s.z})
		}
		if sz1 > iz1+epsilon {
			split = append(split, freeSpace{s.x, s.y, iz1, s.w, s.h, sz1 - iz1})
		}
	}
	e.spaces = append(e.spaces[:n], keepMaximal(e.spaces[:n], split)...)
}

// keepMaximal returns the split spaces that are not contained in another
// space. The unchanged spaces need no check: a split space lies inside the
// space it came from, which contained none of them, so it cannot contain
// one either. Of two equal split spaces, the first is kept.
func keepMaximal(unchanged, split []freeSpace) []freeSpace {
	kept := make([]freeSpace, 0, len(split))
	for i, si := range split {
		contained := false
		for _, sj := range unchanged {
			if contains(sj, si) {
				contained = true
				break
			}
		}
		for j := 0; !contained && j < len(split); j++ {
			if j != i && contains(split[j], si) && (!contains(si, split[j]) || j < i) {
				contained = true
			}
		}
		if !contained {
			kept = append(kept, si)
		}
	}
	return kept
}

// contains reports whether space a contains space b.
func contains(a, b freeSpace) bool {
	return a.x <= b.x+epsilon && a.y <= b.y+epsilon && a.z <= b.z+epsilon &&
		a.x+a.w >= b.x+b.w-epsilon && a.y+a.h >= b.y+b.h-epsilon && a.z+a.d >= b.z+b.d-epsilon
}
