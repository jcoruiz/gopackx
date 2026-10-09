package placement

import (
	"cmp"
	"slices"

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
	rev             uint64              // bin revision the spaces match
	candidates      []maxRectsCandidate // reused between calls
	enableStability bool
	supportRatio    float64
}

// maxRectsCandidate is a spot for an item, with its score (lower is better).
type maxRectsCandidate struct {
	score float64
	rt    model.RotationType
	dim   [3]float64
	pos   [3]float64
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
	// A state for a bin that changed behind the engine's back is dropped.
	if e.bin != nil && e.bin != bin {
		e.saved.save(e.bin, e.spaces, e.rev)
	}
	if spaces, ok := e.saved.load(bin); ok {
		e.bin, e.spaces, e.rev = bin, spaces, bin.Revision()
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
	for k := range bin.Items {
		e.splitSpaces(bin.Box(k))
	}
	e.rev = bin.Revision()
}

// PlaceItem attempts to place an item using maximal rectangles with gravity.
// Candidate spots are tried from the best score down until one passes the
// full placement check (overlap, fragile items, load limits, stability).
func (e *MaxRectsEngine) PlaceItem(bin *model.Bin, item *model.Item) bool {
	if e.bin != bin || e.rev != bin.Revision() {
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

	candidates := e.candidates[:0]

	for si := range e.spaces {
		s := &e.spaces[si]
		for ri := range nRot {
			dim := rds[ri].dim
			if dim[0] > s.w+epsilon || dim[1] > s.h+epsilon || dim[2] > s.d+epsilon {
				continue
			}

			// Apply gravity: drop the item to the items under the space.
			pos := [3]float64{s.x, s.y, s.z}
			gravityY := e.findLowestY(bin, pos[0], pos[1], pos[2], dim)
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
			candidates = append(candidates, maxRectsCandidate{score, rds[ri].rt, dim, pos})
		}
	}
	e.candidates = candidates

	// The best spot is usually valid: try it before sorting the rest. Ties
	// keep the space order (first minimum, stable sort).
	if len(candidates) > 0 {
		best := 0
		for i := range candidates {
			if candidates[i].score < candidates[best].score {
				best = i
			}
		}
		if e.tryCandidate(bin, item, candidates[best]) {
			return true
		}
		candidates = append(candidates[:best], candidates[best+1:]...)
		slices.SortStableFunc(candidates, func(a, b maxRectsCandidate) int { return cmp.Compare(a.score, b.score) })
		for _, c := range candidates {
			if e.tryCandidate(bin, item, c) {
				return true
			}
		}
	}

	item.RotationType = origRT
	item.Position = origPos
	return false
}

// tryCandidate places the item at c if the full placement check passes.
func (e *MaxRectsEngine) tryCandidate(bin *model.Bin, item *model.Item, c maxRectsCandidate) bool {
	item.RotationType = c.rt
	item.Position = c.pos
	if !canPlaceDim(bin, item, c.dim, e.enableStability, e.supportRatio) {
		return false
	}
	bin.PlaceItem(item)
	e.splitSpaces(bin.Box(len(bin.Items) - 1))
	e.rev = bin.Revision()
	return true
}

func (e *MaxRectsEngine) findLowestY(bin *model.Bin, x, y, z float64, dim [3]float64) float64 {
	// The highest top among the items under the footprint that are not
	// above the space: items above it must not lift the item onto them.
	maxY := 0.0
	for k := range bin.Items {
		lo, hi := bin.Box(k)
		if x < hi[0]-epsilon && lo[0] < x+dim[0]-epsilon &&
			z < hi[2]-epsilon && lo[2] < z+dim[2]-epsilon &&
			hi[1] <= y+epsilon && hi[1] > maxY {
			maxY = hi[1]
		}
	}
	return maxY
}

// splitSpaces removes or splits all free spaces that overlap with the placed item.
func (e *MaxRectsEngine) splitSpaces(lo, hi [3]float64) {
	ix0, iy0, iz0 := lo[0], lo[1], lo[2]
	ix1, iy1, iz1 := hi[0], hi[1], hi[2]

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
