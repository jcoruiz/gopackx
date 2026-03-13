package placement

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/intersection"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/stability"
)

const epsilon = 1e-6

// canPlace checks if an item at its current position/rotation is valid in the bin.
func canPlace(bin *model.Bin, item *model.Item, enableStability bool, supportRatio float64) bool {
	dim := item.Dimension()

	// Bin boundaries.
	if item.Position[0]+dim[0] > bin.Width+epsilon ||
		item.Position[1]+dim[1] > bin.Height+epsilon ||
		item.Position[2]+dim[2] > bin.Depth+epsilon {
		return false
	}

	// Negative position check.
	if item.Position[0] < -epsilon || item.Position[1] < -epsilon || item.Position[2] < -epsilon {
		return false
	}

	// Weight capacity.
	if item.Weight > bin.RemainingWeight()+epsilon {
		return false
	}

	// Intersection with placed items.
	for _, placed := range bin.Items {
		if intersection.Intersect(item, placed) {
			return false
		}
	}

	// Stability constraints.
	if enableStability {
		if !stability.CheckSupport(item, bin.Items, supportRatio) {
			return false
		}
	}

	// Fragile constraint: nothing on top of fragile items.
	for _, placed := range bin.Items {
		if !placed.Fragile {
			continue
		}
		placedDim := placed.Dimension()
		placedTop := placed.Position[model.HeightAxis] + placedDim[model.HeightAxis]
		if math.Abs(item.Position[model.HeightAxis]-placedTop) > epsilon {
			continue
		}
		ow := overlapLen(
			item.Position[model.WidthAxis], dim[model.WidthAxis],
			placed.Position[model.WidthAxis], placedDim[model.WidthAxis],
		)
		od := overlapLen(
			item.Position[model.DepthAxis], dim[model.DepthAxis],
			placed.Position[model.DepthAxis], placedDim[model.DepthAxis],
		)
		if ow > epsilon && od > epsilon {
			return false
		}
	}

	// Load-bearing: placing this item must not overload items below.
	if enableStability {
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
	dim := item.Dimension()

	// Process axes in order: Height (gravity), Width, Depth.
	for _, axis := range []int{int(model.HeightAxis), int(model.WidthAxis), int(model.DepthAxis)} {
		maxPos := 0.0

		for _, placed := range bin.Items {
			placedDim := placed.Dimension()

			a1, a2 := otherAxes(axis)
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
