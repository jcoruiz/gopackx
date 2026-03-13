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

// canPlaceDim is the optimized path when item dimensions are pre-computed.
func canPlaceDim(bin *model.Bin, item *model.Item, dim [3]float64, enableStability bool, supportRatio float64) bool {
	iPos := item.Position

	// Bin boundaries.
	if iPos[0]+dim[0] > bin.Width+epsilon ||
		iPos[1]+dim[1] > bin.Height+epsilon ||
		iPos[2]+dim[2] > bin.Depth+epsilon {
		return false
	}

	// Negative position check.
	if iPos[0] < -epsilon || iPos[1] < -epsilon || iPos[2] < -epsilon {
		return false
	}

	// Weight capacity (O(1) using tracked weight).
	if item.Weight > bin.MaxWeight-bin.ItemWeight+epsilon {
		return false
	}

	// Intersection check using cached PlacedDim for placed items.
	ix0, ix1 := iPos[0], iPos[0]+dim[0]
	iy0, iy1 := iPos[1], iPos[1]+dim[1]
	iz0, iz1 := iPos[2], iPos[2]+dim[2]

	for _, placed := range bin.Items {
		pDim := placed.PlacedDim
		pPos := placed.Position

		// Inline AABB intersection (separating axis test).
		if ix0 < pPos[0]+pDim[0]-epsilon && pPos[0] < ix1-epsilon &&
			iy0 < pPos[1]+pDim[1]-epsilon && pPos[1] < iy1-epsilon &&
			iz0 < pPos[2]+pDim[2]-epsilon && pPos[2] < iz1-epsilon {
			return false
		}

		// Fragile constraint: nothing on top of fragile items.
		if placed.Fragile {
			placedTop := pPos[1] + pDim[1]
			if math.Abs(iPos[1]-placedTop) <= epsilon {
				ow := overlapLen(iPos[0], dim[0], pPos[0], pDim[0])
				od := overlapLen(iPos[2], dim[2], pPos[2], pDim[2])
				if ow > epsilon && od > epsilon {
					return false
				}
			}
		}
	}

	// Stability constraints.
	if enableStability {
		if !stability.CheckSupport(item, bin.Items, supportRatio) {
			return false
		}

		// Load-bearing: placing this item must not overload items below.
		// Temporarily add item for CheckLoadBearing (without full PlaceItem).
		bin.Items = append(bin.Items, item)
		for _, placed := range bin.Items {
			if placed == item {
				continue
			}
			if !stability.CheckLoadBearing(placed, bin.Items) {
				bin.Items = bin.Items[:len(bin.Items)-1]
				return false
			}
		}
		bin.Items = bin.Items[:len(bin.Items)-1]
	}

	return true
}

// fixPointDim corrects item position using pre-computed item dimensions.
func fixPointDim(bin *model.Bin, item *model.Item, dim [3]float64) {
	// Process axes in order: Height (gravity), Width, Depth.
	for _, axis := range [3]int{int(model.HeightAxis), int(model.WidthAxis), int(model.DepthAxis)} {
		maxPos := 0.0

		a1, a2 := otherAxes(axis)
		for _, placed := range bin.Items {
			placedDim := placed.PlacedDim

			if !overlaps1D(item.Position[a1], dim[a1], placed.Position[a1], placedDim[a1]) ||
				!overlaps1D(item.Position[a2], dim[a2], placed.Position[a2], placedDim[a2]) {
				continue
			}

			farEdge := placed.Position[axis] + placedDim[axis]
			if farEdge <= item.Position[axis]+epsilon && farEdge > maxPos {
				maxPos = farEdge
			}
		}

		item.Position[axis] = maxPos
	}
}

func otherAxes(axis int) (int, int) {
	switch axis {
	case 0:
		return 1, 2
	case 1:
		return 0, 2
	default:
		return 0, 1
	}
}

func overlaps1D(pos1, len1, pos2, len2 float64) bool {
	return pos1 < pos2+len2-epsilon && pos2 < pos1+len1-epsilon
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
// Pushes the item toward the origin (gravity on Y, compaction on X and Z).
func fixPoint(bin *model.Bin, item *model.Item) {
	fixPointDim(bin, item, item.Dimension())
}
