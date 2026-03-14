// Package intersection provides AABB collision detection for placed items.
package intersection

import "github.com/jcoruiz/gopackx/pkg/model"

const epsilon = 1e-6

// RectIntersect checks if two items overlap when projected onto the plane
// defined by axis1 and axis2. Uses direct AABB edge comparison with epsilon
// tolerance to avoid false positives from floating-point rounding when items
// are exactly touching.
func RectIntersect(i1, i2 *model.Item, axis1, axis2 model.Axis) bool {
	d1 := i1.Dimension()
	d2 := i2.Dimension()

	if i1.Position[axis1] >= i2.Position[axis1]+d2[axis1]-epsilon ||
		i2.Position[axis1] >= i1.Position[axis1]+d1[axis1]-epsilon {
		return false
	}
	if i1.Position[axis2] >= i2.Position[axis2]+d2[axis2]-epsilon ||
		i2.Position[axis2] >= i1.Position[axis2]+d1[axis2]-epsilon {
		return false
	}
	return true
}

// Intersect checks if two placed items overlap in 3D space.
// Two items intersect iff their projections overlap on all three planes (WH, HD, WD).
func Intersect(i1, i2 *model.Item) bool {
	return RectIntersect(i1, i2, model.WidthAxis, model.HeightAxis) &&
		RectIntersect(i1, i2, model.HeightAxis, model.DepthAxis) &&
		RectIntersect(i1, i2, model.WidthAxis, model.DepthAxis)
}
