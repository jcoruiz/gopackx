// Package intersection provides axis-aligned bounding box (AABB) collision
// detection for placed items in 3D bin packing.
//
// Two items occupy non-overlapping space if and only if their projections do
// not overlap on at least one of the three coordinate planes. This package
// uses center-distance comparison for efficient overlap checks.
//
// # Functions
//
// [RectIntersect] checks whether two items overlap when projected onto a 2D
// plane defined by two axes. It computes the center of each item along both
// axes and compares the distance to the sum of half-dimensions.
//
// [Intersect] checks full 3D overlap by verifying that the two items' projections
// overlap on all three planes (width-height, height-depth, and width-depth).
// If any projection does not overlap, the items do not collide.
//
// These functions are used internally by placement engines to validate that a
// new item does not overlap with any already-placed item in a bin.
package intersection
