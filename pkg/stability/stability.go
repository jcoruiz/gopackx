// Package stability provides physical stability checks for item placements.
package stability

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
)

const epsilon = 1e-6

// SupportRatio calculates the fraction of an item's base area that is supported
// by items below it or the bin floor. Returns 1.0 for items on the floor.
func SupportRatio(item *model.Item, placed []*model.Item) float64 {
	if item.Position[model.HeightAxis] < epsilon {
		return 1.0
	}

	dim := item.Dimension()
	baseArea := dim[model.WidthAxis] * dim[model.DepthAxis]
	if baseArea < epsilon {
		return 0
	}

	supportedArea := 0.0
	itemBottom := item.Position[model.HeightAxis]

	for _, other := range placed {
		if other == item {
			continue
		}
		otherDim := other.Dimension()
		otherTop := other.Position[model.HeightAxis] + otherDim[model.HeightAxis]

		if math.Abs(otherTop-itemBottom) > epsilon {
			continue
		}

		overlapW := overlapLength(
			item.Position[model.WidthAxis], dim[model.WidthAxis],
			other.Position[model.WidthAxis], otherDim[model.WidthAxis],
		)
		overlapD := overlapLength(
			item.Position[model.DepthAxis], dim[model.DepthAxis],
			other.Position[model.DepthAxis], otherDim[model.DepthAxis],
		)
		supportedArea += overlapW * overlapD
	}

	return supportedArea / baseArea
}

// CheckSupport returns true if the item has at least the given support ratio.
func CheckSupport(item *model.Item, placed []*model.Item, ratio float64) bool {
	return SupportRatio(item, placed) >= ratio-epsilon
}

// WeightAbove calculates the total weight resting directly on top of an item,
// proportional to the overlap area.
func WeightAbove(item *model.Item, placed []*model.Item) float64 {
	dim := item.Dimension()
	itemTop := item.Position[model.HeightAxis] + dim[model.HeightAxis]

	total := 0.0
	for _, other := range placed {
		if other == item {
			continue
		}
		if math.Abs(other.Position[model.HeightAxis]-itemTop) > epsilon {
			continue
		}
		otherDim := other.Dimension()

		overlapW := overlapLength(
			item.Position[model.WidthAxis], dim[model.WidthAxis],
			other.Position[model.WidthAxis], otherDim[model.WidthAxis],
		)
		overlapD := overlapLength(
			item.Position[model.DepthAxis], dim[model.DepthAxis],
			other.Position[model.DepthAxis], otherDim[model.DepthAxis],
		)
		if overlapW > epsilon && overlapD > epsilon {
			otherBaseArea := otherDim[model.WidthAxis] * otherDim[model.DepthAxis]
			if otherBaseArea > epsilon {
				fraction := (overlapW * overlapD) / otherBaseArea
				total += other.Weight * fraction
			}
		}
	}
	return total
}

// CheckLoadBearing returns true if the item's load-bearing capacity is not exceeded.
// If LoadBear is 0, no limit is enforced.
func CheckLoadBearing(item *model.Item, placed []*model.Item) bool {
	if item.LoadBear <= 0 && !item.Fragile {
		return true
	}
	if item.Fragile {
		return WeightAbove(item, placed) < epsilon
	}
	return WeightAbove(item, placed) <= item.LoadBear+epsilon
}

// GravityCenter calculates weight distribution across 4 quadrants of the bin floor.
// Returns [Q1, Q2, Q3, Q4] as percentages where:
// Q1=front-left, Q2=front-right, Q3=back-left, Q4=back-right.
func GravityCenter(items []*model.Item, binWidth, binDepth float64) [4]float64 {
	if len(items) == 0 {
		return [4]float64{25, 25, 25, 25}
	}

	midW := binWidth / 2
	midD := binDepth / 2
	var qWeight [4]float64
	totalWeight := 0.0

	for _, item := range items {
		dim := item.Dimension()
		w := item.Weight
		totalWeight += w

		posW := item.Position[model.WidthAxis]
		dimW := dim[model.WidthAxis]
		posD := item.Position[model.DepthAxis]
		dimD := dim[model.DepthAxis]

		q1 := quadrantOverlap(posW, dimW, posD, dimD, 0, midW, 0, midD)
		q2 := quadrantOverlap(posW, dimW, posD, dimD, midW, binWidth, 0, midD)
		q3 := quadrantOverlap(posW, dimW, posD, dimD, 0, midW, midD, binDepth)
		q4 := quadrantOverlap(posW, dimW, posD, dimD, midW, binWidth, midD, binDepth)

		totalArea := q1 + q2 + q3 + q4
		if totalArea > epsilon {
			qWeight[0] += w * q1 / totalArea
			qWeight[1] += w * q2 / totalArea
			qWeight[2] += w * q3 / totalArea
			qWeight[3] += w * q4 / totalArea
		}
	}

	if totalWeight < epsilon {
		return [4]float64{25, 25, 25, 25}
	}

	return [4]float64{
		qWeight[0] / totalWeight * 100,
		qWeight[1] / totalWeight * 100,
		qWeight[2] / totalWeight * 100,
		qWeight[3] / totalWeight * 100,
	}
}

func quadrantOverlap(posW, dimW, posD, dimD, qMinW, qMaxW, qMinD, qMaxD float64) float64 {
	ow := overlapLength(posW, dimW, qMinW, qMaxW-qMinW)
	od := overlapLength(posD, dimD, qMinD, qMaxD-qMinD)
	return ow * od
}

func overlapLength(pos1, len1, pos2, len2 float64) float64 {
	start := math.Max(pos1, pos2)
	end := math.Min(pos1+len1, pos2+len2)
	if end <= start {
		return 0
	}
	return end - start
}
