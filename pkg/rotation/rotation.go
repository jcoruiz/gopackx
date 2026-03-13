// Package rotation provides rotation utilities and constraint checking for items.
package rotation

import "github.com/jcoruiz/gopackx/pkg/model"

// matrix maps each RotationType to the index permutation of [width, height, depth].
var matrix = [6][3]int{
	{0, 1, 2}, // WHD
	{1, 0, 2}, // HWD
	{1, 2, 0}, // HDW
	{2, 1, 0}, // DHW
	{2, 0, 1}, // DWH
	{0, 2, 1}, // WDH
}

// Dimensions returns the effective [w, h, d] for the given base dimensions and rotation type.
func Dimensions(w, h, d float64, rt model.RotationType) [3]float64 {
	dims := [3]float64{w, h, d}
	m := matrix[rt]
	return [3]float64{dims[m[0]], dims[m[1]], dims[m[2]]}
}

// DimensionsFor returns the dimensions an item would have under the given rotation,
// without modifying the item's current rotation.
func DimensionsFor(item *model.Item, rt model.RotationType) [3]float64 {
	return Dimensions(item.Width, item.Height, item.Depth, rt)
}

// AllowedFor returns the effective allowed rotations for an item.
// The returned slice must not be modified by the caller.
func AllowedFor(item *model.Item) []model.RotationType {
	if len(item.AllowedRotations) == 0 {
		return model.AllRotations()
	}
	return item.AllowedRotations
}

// IsAllowed checks if a rotation type is permitted for the item.
func IsAllowed(item *model.Item, rt model.RotationType) bool {
	rots := item.AllowedRotations
	if len(rots) == 0 {
		return true // all rotations allowed
	}
	for _, r := range rots {
		if r == rt {
			return true
		}
	}
	return false
}
