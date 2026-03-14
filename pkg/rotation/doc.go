// Package rotation provides rotation utilities and constraint checking for
// 3D bin packing items.
//
// An item can be oriented in 6 possible rotations, each permuting which
// physical dimension (width, height, depth) maps to each spatial axis. This
// package provides functions to compute rotated dimensions, query allowed
// rotations, and check rotation permissions.
//
// # Rotated Dimensions
//
// [Dimensions] computes the effective [width, height, depth] for a given set
// of base dimensions and a rotation type:
//
//	dims := rotation.Dimensions(10, 20, 30, model.RotationHWD)
//	// dims = [20, 10, 30] — height and width are swapped
//
// [DimensionsFor] is a convenience that takes an [model.Item] and a rotation
// type, returning the dimensions the item would have under that rotation
// without modifying the item itself:
//
//	dims := rotation.DimensionsFor(item, model.RotationDWH)
//
// # Allowed Rotations
//
// [AllowedFor] returns a copy of the effective allowed rotations for an item.
// If the item has no explicit allowed rotations set, it defaults to all 6
// rotations.
//
// [IsAllowed] checks whether a specific rotation type is permitted for an item.
package rotation
