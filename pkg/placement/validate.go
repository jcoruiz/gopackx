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
// Uses contiguous AABB data for zero pointer chasing and pre-computed max coordinates.
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

	// Weight capacity.
	if item.Weight > bin.MaxWeight-bin.ItemWeight+epsilon {
		return false
	}

	n := len(bin.Items)
	if n > 0 {
		ix0, ix1 := iPos[0], iPos[0]+dim[0]
		iy0, iy1 := iPos[1], iPos[1]+dim[1]
		iz0, iz1 := iPos[2], iPos[2]+dim[2]

		data := bin.AABBData
		_ = data[n*6-1]

		for i := 0; i < n*6; i += 6 {
			if ix0 < data[i+3]-epsilon && data[i] < ix1-epsilon &&
				iy0 < data[i+4]-epsilon && data[i+1] < iy1-epsilon &&
				iz0 < data[i+5]-epsilon && data[i+2] < iz1-epsilon {
				return false
			}
		}

		if bin.HasFragile {
			for _, fi := range bin.FragileIdxs {
				off := fi * 6
				placedTop := data[off+4]
				if math.Abs(iPos[1]-placedTop) <= epsilon {
					pMinX, pMaxX := data[off], data[off+3]
					pMinZ, pMaxZ := data[off+2], data[off+5]
					ow := overlapLen(iPos[0], dim[0], pMinX, pMaxX-pMinX)
					od := overlapLen(iPos[2], dim[2], pMinZ, pMaxZ-pMinZ)
					if ow > epsilon && od > epsilon {
						return false
					}
				}
			}
		}
	}

	if enableStability {
		if !stability.CheckSupport(item, bin.Items, supportRatio) {
			return false
		}
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

// canPlaceDimBlocker is like canPlaceDim but returns the index of the blocking item
// (or -1 if valid, -2 if bounds/weight/stability failure).
func canPlaceDimBlocker(bin *model.Bin, item *model.Item, dim [3]float64, enableStability bool, supportRatio float64) int {
	iPos := item.Position

	if iPos[0]+dim[0] > bin.Width+epsilon ||
		iPos[1]+dim[1] > bin.Height+epsilon ||
		iPos[2]+dim[2] > bin.Depth+epsilon {
		return -2
	}
	if iPos[0] < -epsilon || iPos[1] < -epsilon || iPos[2] < -epsilon {
		return -2
	}
	if item.Weight > bin.MaxWeight-bin.ItemWeight+epsilon {
		return -2
	}

	n := len(bin.Items)
	if n > 0 {
		ix0, ix1 := iPos[0], iPos[0]+dim[0]
		iy0, iy1 := iPos[1], iPos[1]+dim[1]
		iz0, iz1 := iPos[2], iPos[2]+dim[2]

		data := bin.AABBData
		_ = data[n*6-1]

		for i := 0; i < n*6; i += 6 {
			if ix0 < data[i+3]-epsilon && data[i] < ix1-epsilon &&
				iy0 < data[i+4]-epsilon && data[i+1] < iy1-epsilon &&
				iz0 < data[i+5]-epsilon && data[i+2] < iz1-epsilon {
				return i / 6
			}
		}

		if bin.HasFragile {
			for _, fi := range bin.FragileIdxs {
				off := fi * 6
				placedTop := data[off+4]
				if math.Abs(iPos[1]-placedTop) <= epsilon {
					pMinX, pMaxX := data[off], data[off+3]
					pMinZ, pMaxZ := data[off+2], data[off+5]
					ow := overlapLen(iPos[0], dim[0], pMinX, pMaxX-pMinX)
					od := overlapLen(iPos[2], dim[2], pMinZ, pMaxZ-pMinZ)
					if ow > epsilon && od > epsilon {
						return fi
					}
				}
			}
		}
	}

	if enableStability {
		if !stability.CheckSupport(item, bin.Items, supportRatio) {
			return -2
		}
		bin.Items = append(bin.Items, item)
		for _, placed := range bin.Items {
			if placed == item {
				continue
			}
			if !stability.CheckLoadBearing(placed, bin.Items) {
				bin.Items = bin.Items[:len(bin.Items)-1]
				return -2
			}
		}
		bin.Items = bin.Items[:len(bin.Items)-1]
	}

	return -1 // valid
}

// fixPointDim corrects item position using pre-computed AABB data.
func fixPointDim(bin *model.Bin, item *model.Item, dim [3]float64) {
	n := len(bin.Items)
	if n == 0 {
		item.Position = [3]float64{0, 0, 0}
		return
	}

	data := bin.AABBData
	_ = data[n*6-1]

	for _, axis := range [3]int{int(model.HeightAxis), int(model.WidthAxis), int(model.DepthAxis)} {
		maxPos := 0.0
		a1, a2 := otherAxes(axis)

		for i := 0; i < n*6; i += 6 {
			pMin1, pMax1 := data[i+a1], data[i+3+a1]
			pMin2, pMax2 := data[i+a2], data[i+3+a2]

			if !overlaps1D(item.Position[a1], dim[a1], pMin1, pMax1-pMin1) ||
				!overlaps1D(item.Position[a2], dim[a2], pMin2, pMax2-pMin2) {
				continue
			}

			farEdge := data[i+3+axis]
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

// canPlaceStability checks only the stability constraints.
func canPlaceStability(bin *model.Bin, item *model.Item, supportRatio float64) bool {
	if !stability.CheckSupport(item, bin.Items, supportRatio) {
		return false
	}
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
	return true
}

// fixPoint corrects item position on each axis to eliminate floating gaps.
func fixPoint(bin *model.Bin, item *model.Item) {
	fixPointDim(bin, item, item.Dimension())
}
