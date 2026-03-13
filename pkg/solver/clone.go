package solver

import "github.com/jcoruiz/gopackx/pkg/model"

// resetItem creates a deep copy of an item with placement state cleared.
func resetItem(item *model.Item) *model.Item {
	c := *item
	c.AllowedRotations = make([]model.RotationType, len(item.AllowedRotations))
	copy(c.AllowedRotations, item.AllowedRotations)
	c.Placed = false
	c.Position = [3]float64{}
	c.PlacedDim = [3]float64{}
	c.RotationType = 0
	return &c
}

// resetItems creates deep copies of all items with placement state cleared.
func resetItems(items []*model.Item) []*model.Item {
	out := make([]*model.Item, len(items))
	for i, item := range items {
		out[i] = resetItem(item)
	}
	return out
}

// cloneBinEmpty creates a copy of a bin with no items.
func cloneBinEmpty(bin *model.Bin) *model.Bin {
	c := *bin
	c.Items = nil
	c.UnfittedItems = nil
	c.ItemWeight = 0
	c.ItemVolume = 0
	return &c
}

// snapshotBin creates a full deep copy of a bin including all placed items.
func snapshotBin(bin *model.Bin) *model.Bin {
	c := *bin
	c.Items = make([]*model.Item, len(bin.Items))
	for i, item := range bin.Items {
		cp := *item
		cp.AllowedRotations = make([]model.RotationType, len(item.AllowedRotations))
		copy(cp.AllowedRotations, item.AllowedRotations)
		c.Items[i] = &cp
	}
	c.UnfittedItems = nil
	return &c
}
