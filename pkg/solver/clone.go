package solver

import "github.com/jcoruiz/gopackx/pkg/model"

// resetItem creates a deep copy of an item with placement state cleared.
func resetItem(item *model.Item) *model.Item {
	c := item.Clone()
	c.ResetPlacement()
	return c
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
	return bin.CloneEmpty()
}

// snapshotBin creates a full deep copy of a bin including all placed items.
func snapshotBin(bin *model.Bin) *model.Bin {
	return bin.Clone()
}
