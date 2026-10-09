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

// Resetter is implemented by every engine in this package. Engines keep
// per-bin state while packing; Reset forgets it so the engine can be reused
// for an unrelated packing run.
type Resetter interface {
	Reset()
}
