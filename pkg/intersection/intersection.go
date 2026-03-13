// Package intersection provides AABB collision detection for placed items.
package intersection

import (
	"math"

	"github.com/jcoruiz/gopackx/pkg/model"
)

// RectIntersect checks if two items overlap when projected onto the plane
// defined by axis1 and axis2. Uses center-distance comparison.
func RectIntersect(i1, i2 *model.Item, axis1, axis2 model.Axis) bool {
	d1 := i1.Dimension()
	d2 := i2.Dimension()

	cx1 := i1.Position[axis1] + d1[axis1]/2
	cx2 := i2.Position[axis1] + d2[axis1]/2
	cy1 := i1.Position[axis2] + d1[axis2]/2
	cy2 := i2.Position[axis2] + d2[axis2]/2

	ix := math.Abs(cx1-cx2) < (d1[axis1]+d2[axis1])/2
	iy := math.Abs(cy1-cy2) < (d1[axis2]+d2[axis2])/2

	return ix && iy
}

// Intersect checks if two placed items overlap in 3D space.
// Two items intersect iff their projections overlap on all three planes (WH, HD, WD).
func Intersect(i1, i2 *model.Item) bool {
	return RectIntersect(i1, i2, model.WidthAxis, model.HeightAxis) &&
		RectIntersect(i1, i2, model.HeightAxis, model.DepthAxis) &&
		RectIntersect(i1, i2, model.WidthAxis, model.DepthAxis)
}
