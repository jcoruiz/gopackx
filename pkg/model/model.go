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

// AllRotations contains all 6 possible rotation types.
var AllRotations = []RotationType{
	RotationWHD, RotationHWD, RotationHDW,
	RotationDHW, RotationDWH, RotationWDH,
}

// UprightRotations contains only rotations that keep the original height axis vertical.
var UprightRotations = []RotationType{
	RotationWHD, RotationDHW,
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
}

// NewItem creates a new Item with precalculated volume.
func NewItem(id string, w, h, d, weight float64) *Item {
	return &Item{
		ID:               id,
		Width:            w,
		Height:           h,
		Depth:            d,
		Weight:           weight,
		Volume:           w * h * d,
		AllowedRotations: AllRotations,
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

// TotalWeight returns the sum of weights of all placed items.
func (b *Bin) TotalWeight() float64 {
	total := 0.0
	for _, item := range b.Items {
		total += item.Weight
	}
	return total
}

// RemainingWeight returns how much weight capacity is left.
func (b *Bin) RemainingWeight() float64 {
	return b.MaxWeight - b.TotalWeight()
}

// UsedVolume returns the sum of volumes of all placed items.
func (b *Bin) UsedVolume() float64 {
	total := 0.0
	for _, item := range b.Items {
		total += item.Volume
	}
	return total
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
