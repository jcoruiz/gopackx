// Package placement provides item placement engines (pivot points, extreme points).
package placement

import "github.com/jcoruiz/gopackx/pkg/model"

// Engine defines the interface for item placement algorithms.
type Engine interface {
	// PlaceItem attempts to place an item in the bin. Returns true if successful.
	// On success, the item's Position, RotationType, and Placed fields are set,
	// and the item is appended to bin.Items.
	PlaceItem(bin *model.Bin, item *model.Item) bool
}
