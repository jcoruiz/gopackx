// Package model defines the core data types for 3D bin packing.
package model

// Axis represents a spatial axis.
type Axis int

const (
	WidthAxis  Axis = 0
	HeightAxis Axis = 1
	DepthAxis  Axis = 2
)

// RotationType represents one of 6 possible orientations of an item.
type RotationType int

const (
	RotationWHD RotationType = 0 // width, height, depth (default)
	RotationHWD RotationType = 1 // height, width, depth
	RotationHDW RotationType = 2 // height, depth, width
	RotationDHW RotationType = 3 // depth, height, width
	RotationDWH RotationType = 4 // depth, width, height
	RotationWDH RotationType = 5 // width, depth, height
)

// allRotations is the canonical list of all 6 rotation types.
var allRotations = [6]RotationType{
	RotationWHD, RotationHWD, RotationHDW,
	RotationDHW, RotationDWH, RotationWDH,
}

// uprightRotations is the canonical list of upright-only rotations.
var uprightRotations = [2]RotationType{
	RotationWHD, RotationDHW,
}

// Shared slices backed by the canonical arrays. Must not be modified.
var allRotationsSlice = allRotations[:]
var uprightRotationsSlice = uprightRotations[:]

// AllRotations returns all 6 possible rotation types.
// The returned slice must not be modified by the caller.
func AllRotations() []RotationType {
	return allRotationsSlice
}

// UprightRotations returns rotations that keep the height axis vertical.
// The returned slice must not be modified by the caller.
func UprightRotations() []RotationType {
	return uprightRotationsSlice
}

// rotationMatrix maps each RotationType to the index permutation of [width, height, depth].
var rotationMatrix = [6][3]int{
	{0, 1, 2}, // WHD
	{1, 0, 2}, // HWD
	{1, 2, 0}, // HDW
	{2, 1, 0}, // DHW
	{2, 0, 1}, // DWH
	{0, 2, 1}, // WDH
}

// Item represents an object to be packed into a bin.
type Item struct {
	ID               string
	Width            float64
	Height           float64
	Depth            float64
	Weight           float64
	Volume           float64
	RotationType     RotationType
	Position         [3]float64
	AllowedRotations []RotationType
	Priority         int
	LoadBear         float64
	Fragile          bool
	Group            string
	Placed           bool
	PlacedDim        [3]float64 // cached dimensions after placement
}

// ItemOption configures optional fields on an Item.
type ItemOption func(*Item)

// NewItem creates a new Item with precalculated volume.
func NewItem(id string, w, h, d, weight float64, opts ...ItemOption) *Item {
	item := &Item{
		ID:               id,
		Width:            w,
		Height:           h,
		Depth:            d,
		Weight:           weight,
		Volume:           w * h * d,
		AllowedRotations: AllRotations(),
	}
	for _, opt := range opts {
		opt(item)
	}
	return item
}

// ItemUpright restricts the item to rotations that keep the height axis vertical.
func ItemUpright() ItemOption {
	return func(i *Item) { i.AllowedRotations = UprightRotations() }
}

// ItemPriority sets the packing priority (1=highest).
func ItemPriority(p int) ItemOption {
	return func(i *Item) { i.Priority = p }
}

// ItemLoadBear sets the maximum weight the item can support on top.
func ItemLoadBear(lb float64) ItemOption {
	return func(i *Item) { i.LoadBear = lb }
}

// ItemFragile marks the item as fragile (nothing can be placed on top).
func ItemFragile() ItemOption {
	return func(i *Item) { i.Fragile = true }
}

// ItemGroup assigns the item to a binding group.
func ItemGroup(g string) ItemOption {
	return func(i *Item) { i.Group = g }
}

// ItemAllowedRotations sets custom allowed rotations.
func ItemAllowedRotations(rots []RotationType) ItemOption {
	return func(i *Item) {
		i.AllowedRotations = make([]RotationType, len(rots))
		copy(i.AllowedRotations, rots)
	}
}

// Dimension returns the effective [w, h, d] after applying the current rotation.
func (it *Item) Dimension() [3]float64 {
	dims := [3]float64{it.Width, it.Height, it.Depth}
	m := rotationMatrix[it.RotationType]
	return [3]float64{dims[m[0]], dims[m[1]], dims[m[2]]}
}

// Bin represents a container that items are packed into.
type Bin struct {
	ID            string
	Width         float64
	Height        float64
	Depth         float64
	MaxWeight     float64
	Volume        float64
	Items         []*Item
	UnfittedItems []*Item
	ItemWeight    float64 // tracked sum of item weights
	ItemVolume    float64 // tracked sum of item volumes
}

// NewBin creates a new Bin with precalculated volume.
func NewBin(id string, w, h, d, maxWeight float64) *Bin {
	return &Bin{
		ID:        id,
		Width:     w,
		Height:    h,
		Depth:     d,
		MaxWeight: maxWeight,
		Volume:    w * h * d,
	}
}

// PlaceItem adds an item to the bin and updates tracked weight/volume.
func (b *Bin) PlaceItem(item *Item) {
	item.Placed = true
	item.PlacedDim = item.Dimension()
	b.Items = append(b.Items, item)
	b.ItemWeight += item.Weight
	b.ItemVolume += item.Volume
}

// RemoveLastItem removes the last placed item and updates tracked weight/volume.
func (b *Bin) RemoveLastItem() *Item {
	n := len(b.Items)
	item := b.Items[n-1]
	b.Items = b.Items[:n-1]
	b.ItemWeight -= item.Weight
	b.ItemVolume -= item.Volume
	item.Placed = false
	return item
}

// TotalWeight returns the sum of weights of all placed items.
func (b *Bin) TotalWeight() float64 {
	return b.ItemWeight
}

// RemainingWeight returns how much weight capacity is left.
func (b *Bin) RemainingWeight() float64 {
	return b.MaxWeight - b.ItemWeight
}

// UsedVolume returns the sum of volumes of all placed items.
func (b *Bin) UsedVolume() float64 {
	return b.ItemVolume
}

// VolumeUsedPct returns the percentage of bin volume occupied by items.
func (b *Bin) VolumeUsedPct() float64 {
	if b.Volume == 0 {
		return 0
	}
	return (b.UsedVolume() / b.Volume) * 100
}

// PackingStats contains summary statistics for a packing result.
type PackingStats struct {
	TotalBins     int
	TotalItems    int
	FittedItems   int
	UnfittedCount int
	VolumeUsedPct float64
	WeightUsedPct float64
}

// Result holds the outcome of a packing operation.
type Result struct {
	Bins          []*Bin
	UnfittedItems []*Item
	Stats         PackingStats
}
