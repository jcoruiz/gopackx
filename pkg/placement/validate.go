package placement

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/stability"
)

const epsilon = 1e-6

// canPlace checks if an item at its current position/rotation is valid in the bin.
func canPlace(bin *model.Bin, item *model.Item, enableStability bool, supportRatio float64) bool {
	return canPlaceDim(bin, item, item.Dimension(), enableStability, supportRatio)
}

// canPlaceDim checks if an item at its current position, with the given
// rotated dimensions, is a valid placement.
func canPlaceDim(bin *model.Bin, item *model.Item, dim [3]float64, enableStability bool, supportRatio float64) bool {
	return canPlaceDimBlocker(bin, item, dim, enableStability, supportRatio) == -1
}

// canPlaceDimBlocker checks a placement. It returns -1 if it is valid, the
// index of the item that blocks it (an overlap, or a fragile item it would
// rest on), or -2 for any other reason: bounds, weight, load limits or
// support.
//
// Fragile and load limits are checked in both directions: the item must
// not rest on a fragile item or overload the items under it, and it must
// not be slid under items that would rest on it if it is fragile or would
// carry more than it can hold.
func canPlaceDimBlocker(bin *model.Bin, item *model.Item, dim [3]float64, enableStability bool, supportRatio float64) int {
	pos := item.Position

	if pos[0]+dim[0] > bin.Width+epsilon ||
		pos[1]+dim[1] > bin.Height+epsilon ||
		pos[2]+dim[2] > bin.Depth+epsilon {
		return -2
	}
	if pos[0] < -epsilon || pos[1] < -epsilon || pos[2] < -epsilon {
		return -2
	}
	if !bin.CanCarry(item.Weight) {
		return -2
	}

	if k := bin.Collides(pos, dim); k >= 0 {
		return k
	}
	if k := bin.RestsOnFragile(pos, dim); k >= 0 {
		return k
	}
	if item.Fragile && bin.HasItemOnTop(pos, dim) {
		return -2
	}
	if !bin.FitsLoadLimits(item, pos, dim) {
		return -2
	}
	if enableStability && !stability.CheckSupport(item, bin.Items, supportRatio) {
		return -2
	}
	return -1
}

// fixPointDim moves an item toward the origin, height axis first, until it
// rests against the items or walls it overlaps with seen along that axis.
func fixPointDim(bin *model.Bin, item *model.Item, dim [3]float64) {
	item.Position = bin.SlideToOrigin(item.Position, dim)
}

func overlapLen(pos1, len1, pos2, len2 float64) float64 {
	start := math.Max(pos1, pos2)
	end := math.Min(pos1+len1, pos2+len2)
	if end <= start {
		return 0
	}
	return end - start
}

// fixPoint corrects item position on each axis to eliminate floating gaps.
func fixPoint(bin *model.Bin, item *model.Item) {
	fixPointDim(bin, item, item.Dimension())
}

// blockers remembers the last few items that blocked a candidate. Most
// candidates collide with one of them, which is cheaper to test than every
// item in the bin. A hit always means a real overlap, so skipping it never
// rejects a valid spot.
type blockers struct {
	boxes [4][6]float64
	n     int
	next  int
}

// hit reports whether a box at pos with size dim overlaps a remembered
// blocker, with the same tolerance as canPlaceDim.
func (b *blockers) hit(pos, dim [3]float64) bool {
	x1, y1, z1 := pos[0]+dim[0], pos[1]+dim[1], pos[2]+dim[2]
	for i := range b.n {
		o := &b.boxes[i]
		if pos[0] < o[3]-epsilon && o[0] < x1-epsilon &&
			pos[1] < o[4]-epsilon && o[1] < y1-epsilon &&
			pos[2] < o[5]-epsilon && o[2] < z1-epsilon {
			return true
		}
	}
	return false
}

// add remembers the item at index idx of the bin as a blocker.
func (b *blockers) add(bin *model.Bin, idx int) {
	lo, hi := bin.Box(idx)
	b.boxes[b.next] = [6]float64{lo[0], lo[1], lo[2], hi[0], hi[1], hi[2]}
	b.next = (b.next + 1) & (len(b.boxes) - 1) // len is a power of two
	if b.n < len(b.boxes) {
		b.n++
	}
}
