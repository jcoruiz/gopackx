// Package model defines the core data types for 3D bin packing.
package model

import (
	"math"
	"slices"
)

// Axis represents a spatial axis.
type Axis int

// Spatial axes, used to index [width, height, depth] dimension arrays.
const (
	WidthAxis  Axis = 0
	HeightAxis Axis = 1
	DepthAxis  Axis = 2
)

// RotationType represents one of 6 possible orientations of an item.
type RotationType int

// Rotation types. Each name lists which physical dimension maps to the
// width, height and depth axes, in that order.
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

// Clone returns a deep copy of the item, placement included.
func (it *Item) Clone() *Item {
	c := *it
	c.AllowedRotations = slices.Clone(it.AllowedRotations)
	return &c
}

// ResetPlacement clears the placement of the item: position, rotation and
// the Placed flag.
func (it *Item) ResetPlacement() {
	it.Placed = false
	it.Position = [3]float64{}
	it.RotationType = RotationWHD
}

// Dimension returns the effective [w, h, d] after applying the current rotation.
func (it *Item) Dimension() [3]float64 {
	dims := [3]float64{it.Width, it.Height, it.Depth}
	m := rotationMatrix[it.RotationType]
	return [3]float64{dims[m[0]], dims[m[1]], dims[m[2]]}
}

// Bin represents a container that items are packed into.
type Bin struct {
	ID string
	// TypeID is the ID of the box type a solver opened this bin from
	// (TrialPacking, Metaheuristic, gopackx.Pack). It is empty for bins
	// created with NewBin.
	TypeID    string
	Width     float64
	Height    float64
	Depth     float64
	MaxWeight float64 // 0 means no weight limit
	Cost      float64 // cost per bin; 0 means unset (solvers minimize bin count instead)
	Volume    float64
	// Items placed in the bin, in placement order. Change them only with
	// PlaceItem and RemoveLastItem, which keep the bin's tracked data
	// (weight, positions, loads) in sync.
	Items []*Item

	weight  float64     // sum of item weights
	volume  float64     // sum of item volumes
	boxes   []float64   // per item: x0, y0, z0, x1, y1, z1
	fragile []int       // indexes of fragile items
	limited int         // number of items with a load limit
	tracked bool        // loads and loadLog are kept for every item
	loads   []float64   // per item: weight resting on it, everything above counted
	loadLog [][]loadAdd // per item: the load it added to the items below
}

// loadAdd records load added to item idx.
type loadAdd struct {
	idx int
	w   float64
}

// BinOption configures optional fields on a Bin.
type BinOption func(*Bin)

// BinCost sets the cost per bin unit. When set, solvers minimize total cost
// instead of total bin count.
func BinCost(cost float64) BinOption {
	return func(b *Bin) { b.Cost = cost }
}

// NewBin creates a new Bin with precalculated volume.
// A maxWeight of 0 means the bin has no weight limit.
func NewBin(id string, w, h, d, maxWeight float64, opts ...BinOption) *Bin {
	b := &Bin{
		ID:        id,
		Width:     w,
		Height:    h,
		Depth:     d,
		MaxWeight: maxWeight,
		Volume:    w * h * d,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// CloneEmpty returns a bin with the same configuration and no items.
func (b *Bin) CloneEmpty() *Bin {
	return &Bin{
		ID:        b.ID,
		TypeID:    b.TypeID,
		Width:     b.Width,
		Height:    b.Height,
		Depth:     b.Depth,
		MaxWeight: b.MaxWeight,
		Cost:      b.Cost,
		Volume:    b.Volume,
	}
}

// Clone returns a deep copy of the bin, with copies of the items placed in it.
func (b *Bin) Clone() *Bin {
	c := *b
	c.Items = make([]*Item, len(b.Items))
	for i, it := range b.Items {
		c.Items[i] = it.Clone()
	}
	c.boxes = slices.Clone(b.boxes)
	c.fragile = slices.Clone(b.fragile)
	c.loads = slices.Clone(b.loads)
	c.loadLog = slices.Clone(b.loadLog) // entries are never modified
	return &c
}

// PlaceItem adds an item to the bin at its current position and rotation,
// and updates the tracked weight, volume and loads. It does not check that
// the item fits: placement engines do.
func (b *Bin) PlaceItem(item *Item) {
	item.Placed = true
	dim := item.Dimension()
	lo := item.Position
	hi := [3]float64{lo[0] + dim[0], lo[1] + dim[1], lo[2] + dim[2]}

	if item.LoadBear > 0 {
		b.limited++
		b.trackLoads()
	}
	if !b.tracked {
		b.appendBox(item, lo, hi)
		return
	}

	// Items placed earlier may rest on this one (it was slid under them):
	// part of their weight moves from their other supports to it.
	onTop, moved := b.loadAbove(lo, hi)
	b.appendBox(item, lo, hi)
	b.loads = append(b.loads, onTop)
	_, added := b.passDown(moved, b.shareDown(nil, lo, hi, item.Weight+onTop), true)
	b.loadLog = append(b.loadLog, added)
}

func (b *Bin) appendBox(item *Item, lo, hi [3]float64) {
	b.Items = append(b.Items, item)
	b.boxes = append(b.boxes, lo[0], lo[1], lo[2], hi[0], hi[1], hi[2])
	b.weight += item.Weight
	b.volume += item.Volume
	if item.Fragile {
		b.fragile = append(b.fragile, len(b.Items)-1)
	}
}

// trackLoads starts keeping loads, computing them for the items already
// placed as if they had been tracked from the start. Bins without load
// limits never need loads, so they skip this work.
func (b *Bin) trackLoads() {
	if b.tracked {
		return
	}
	items, boxes := b.Items, b.boxes
	b.tracked = true
	b.loads = make([]float64, 0, len(items))
	b.loadLog = make([][]loadAdd, 0, len(items))
	for k := range items {
		// Replay item k against the items placed before it.
		b.Items, b.boxes = items[:k], boxes[:6*k]
		lo := [3]float64{boxes[6*k], boxes[6*k+1], boxes[6*k+2]}
		hi := [3]float64{boxes[6*k+3], boxes[6*k+4], boxes[6*k+5]}
		onTop, moved := b.loadAbove(lo, hi)
		b.Items, b.boxes = items[:k+1], boxes[:6*(k+1)]
		b.loads = append(b.loads, onTop)
		_, added := b.passDown(moved, b.shareDown(nil, lo, hi, items[k].Weight+onTop), true)
		b.loadLog = append(b.loadLog, added)
	}
	b.Items, b.boxes = items, boxes
}

// RemoveLastItem removes the last placed item and updates tracked weight/volume.
func (b *Bin) RemoveLastItem() *Item {
	n := len(b.Items)
	item := b.Items[n-1]
	if b.tracked {
		for _, a := range b.loadLog[n-1] {
			b.loads[a.idx] -= a.w
		}
		b.loads = b.loads[:n-1]
		b.loadLog = b.loadLog[:n-1]
	}
	b.Items = b.Items[:n-1]
	b.boxes = b.boxes[:6*(n-1)]
	b.weight -= item.Weight
	b.volume -= item.Volume
	if item.Fragile {
		b.fragile = b.fragile[:len(b.fragile)-1]
	}
	if item.LoadBear > 0 {
		b.limited--
	}
	item.Placed = false
	return item
}

// Box returns the corners of the space taken by item i: lo is the corner
// closest to the origin, hi the opposite one.
func (b *Bin) Box(i int) (lo, hi [3]float64) {
	o := b.boxes[6*i : 6*i+6]
	return [3]float64{o[0], o[1], o[2]}, [3]float64{o[3], o[4], o[5]}
}

// Collides returns the index of an item that overlaps a box placed at pos
// with size dim, or -1 if none does. Touching faces do not overlap.
func (b *Bin) Collides(pos, dim [3]float64) int {
	x1, y1, z1 := pos[0]+dim[0], pos[1]+dim[1], pos[2]+dim[2]
	// Reslicing six at a time lets the compiler drop bounds checks.
	for k, o := 0, b.boxes; len(o) >= 6; k, o = k+1, o[6:] {
		if pos[0] < o[3]-spaceTolerance && o[0] < x1-spaceTolerance &&
			pos[1] < o[4]-spaceTolerance && o[1] < y1-spaceTolerance &&
			pos[2] < o[5]-spaceTolerance && o[2] < z1-spaceTolerance {
			return k
		}
	}
	return -1
}

// SlideToOrigin returns where a box at pos with size dim ends up when pushed
// toward the origin, height axis first, then width, then depth: on each axis
// it stops at the far face of the closest item it would run into, or at the
// wall.
func (b *Bin) SlideToOrigin(pos, dim [3]float64) [3]float64 {
	for _, axis := range [3]int{1, 0, 2} {
		a1, a2 := (axis+1)%3, (axis+2)%3
		lo1, hi1 := pos[a1], pos[a1]+dim[a1]
		lo2, hi2 := pos[a2], pos[a2]+dim[a2]
		stop := 0.0
		for o := b.boxes; len(o) >= 6; o = o[6:] {
			box := (*[6]float64)(o)
			if lo1 < box[3+a1]-spaceTolerance && box[a1] < hi1-spaceTolerance &&
				lo2 < box[3+a2]-spaceTolerance && box[a2] < hi2-spaceTolerance {
				if far := box[3+axis]; far <= pos[axis]+spaceTolerance && far > stop {
					stop = far
				}
			}
		}
		pos[axis] = stop
	}
	return pos
}

// RestsOnFragile returns the index of a fragile item that a box placed at
// pos with size dim would rest on, or -1.
func (b *Bin) RestsOnFragile(pos, dim [3]float64) int {
	for _, f := range b.fragile {
		o := b.boxes[6*f : 6*f+6]
		if restsOn(pos[1], o[1], o[4]) && footprint(pos, dim, o) > spaceTolerance {
			return f
		}
	}
	return -1
}

// HasItemOnTop reports whether an item rests on the top face of a box
// placed at pos with size dim.
func (b *Bin) HasItemOnTop(pos, dim [3]float64) bool {
	top := pos[1] + dim[1]
	for o := b.boxes; len(o) >= 6; o = o[6:] {
		if restsOn(o[1], pos[1], top) && footprint(pos, dim, o) > spaceTolerance {
			return true
		}
	}
	return false
}

// Load returns the weight resting on item i: everything stacked on it. Each
// item passes its own weight plus the load on it to the items directly
// under it, split in proportion to how much of its base rests on each.
func (b *Bin) Load(i int) float64 {
	b.trackLoads()
	return b.loads[i]
}

// FitsLoadLimits reports whether item, placed at pos with size dim, keeps
// every load limit in the bin: its own, for items it would be slid under,
// and those of every item below it, which would carry its weight.
func (b *Bin) FitsLoadLimits(item *Item, pos, dim [3]float64) bool {
	if len(b.Items) == 0 || (b.limited == 0 && item.LoadBear <= 0) {
		return true
	}
	b.trackLoads()
	hi := [3]float64{pos[0] + dim[0], pos[1] + dim[1], pos[2] + dim[2]}
	onTop, moved := b.loadAbove(pos, hi)
	if item.LoadBear > 0 && onTop > item.LoadBear+weightTolerance {
		return false
	}
	if b.limited == 0 {
		return true
	}
	ok, _ := b.passDown(moved, b.shareDown(nil, pos, hi, item.Weight+onTop), false)
	return ok
}

// spaceTolerance absorbs floating point error in positions.
const spaceTolerance = 1e-6

// footprint returns the area where a box at pos with size dim and the
// placed box o (x0, y0, z0, x1, y1, z1) overlap seen from above.
func footprint(pos, dim [3]float64, o []float64) float64 {
	return overlap1D(pos[0], pos[0]+dim[0], o[0], o[3]) * overlap1D(pos[2], pos[2]+dim[2], o[2], o[5])
}

func overlap1D(a0, a1, b0, b1 float64) float64 {
	return math.Max(0, math.Min(a1, b1)-math.Max(a0, b0))
}

// loadAbove returns the weight the placed items would put on a new box
// spanning lo to hi. An item resting on the box moves part of its weight
// (plus its own load) from its current supports to the box; moved holds
// those (negative) changes for the current supports.
func (b *Bin) loadAbove(lo, hi [3]float64) (onTop float64, moved []loadAdd) {
	o := b.boxes
	for i := 0; i < len(o); i += 6 {
		if !restsOn(o[i+1], lo[1], hi[1]) {
			continue
		}
		touch := overlap1D(lo[0], hi[0], o[i], o[i+3]) * overlap1D(lo[2], hi[2], o[i+2], o[i+5])
		if touch <= spaceTolerance {
			continue
		}
		k := i / 6
		carried := b.Items[k].Weight + b.loads[k]
		klo, khi := b.Box(k)
		sup := b.supports(nil, klo, khi)
		area := 0.0
		for _, s := range sup {
			area += s.w
		}
		onTop += carried * touch / (area + touch)
		for _, s := range sup {
			moved = append(moved, loadAdd{s.idx, carried * (s.w/(area+touch) - s.w/area)})
		}
	}
	return onTop, moved
}

// supports appends to dst, for every item whose top face touches the
// bottom of a box spanning lo to hi, its index and the contact area.
func (b *Bin) supports(dst []loadAdd, lo, hi [3]float64) []loadAdd {
	if lo[1] <= spaceTolerance {
		return dst // on the floor
	}
	o := b.boxes
	for i := 0; i < len(o); i += 6 {
		if !restsOn(lo[1], o[i+1], o[i+4]) {
			continue
		}
		touch := overlap1D(lo[0], hi[0], o[i], o[i+3]) * overlap1D(lo[2], hi[2], o[i+2], o[i+5])
		if touch > spaceTolerance {
			dst = append(dst, loadAdd{i / 6, touch})
		}
	}
	return dst
}

// restsOn reports whether something whose bottom is at bottom rests on a box
// spanning underBottom to underTop: the bottom touches the box's top face
// and is strictly above the box's own bottom. The second condition keeps a
// box thinner than the tolerance from supporting itself or an item on the
// same level, so load always flows strictly down.
func restsOn(bottom, underBottom, underTop float64) bool {
	return math.Abs(bottom-underTop) <= spaceTolerance && bottom > underBottom
}

// shareDown appends to dst the load w, carried by a box spanning lo to hi,
// split among the items under it in proportion to their contact areas.
func (b *Bin) shareDown(dst []loadAdd, lo, hi [3]float64, w float64) []loadAdd {
	start := len(dst)
	dst = b.supports(dst, lo, hi)
	area := 0.0
	for _, s := range dst[start:] {
		area += s.w
	}
	for i := start; i < len(dst); i++ {
		dst[i].w = w * dst[i].w / area
	}
	return dst
}

// passDown adds load changes to items and passes each change on down to
// the items that support them. moved holds the changes for supports that
// give up part of an item resting on a new box, start the new box's share
// for the items under it. With apply it updates the loads and returns every
// change made; otherwise it only reports whether every load limit holds.
func (b *Bin) passDown(moved, start []loadAdd, apply bool) (bool, []loadAdd) {
	var pending []loadAdd
	add := func(k int, w float64) {
		for i := range pending {
			if pending[i].idx == k {
				pending[i].w += w
				return
			}
		}
		pending = append(pending, loadAdd{k, w})
	}
	for _, c := range moved {
		add(c.idx, c.w)
	}
	for _, c := range start {
		add(c.idx, c.w)
	}

	// Load only flows down, so handling items from the highest bottom face
	// down means each one has received all its change before passing it on.
	var added, below []loadAdd
	for len(pending) > 0 {
		next := 0
		for i := range pending {
			if b.boxes[6*pending[i].idx+1] > b.boxes[6*pending[next].idx+1] {
				next = i
			}
		}
		cur := pending[next]
		pending = append(pending[:next], pending[next+1:]...)

		if apply {
			b.loads[cur.idx] += cur.w
			added = append(added, cur)
		} else if lb := b.Items[cur.idx].LoadBear; lb > 0 && cur.w > 0 && b.loads[cur.idx]+cur.w > lb+weightTolerance {
			return false, nil
		}
		klo, khi := b.Box(cur.idx)
		below = b.shareDown(below[:0], klo, khi, cur.w)
		for _, c := range below {
			add(c.idx, c.w)
		}
	}
	return true, added
}

// TotalWeight returns the sum of weights of all placed items.
func (b *Bin) TotalWeight() float64 {
	return b.weight
}

// weightTolerance absorbs floating point error in weight sums.
const weightTolerance = 1e-6

// HasWeightLimit reports whether the bin limits the weight it carries.
// A MaxWeight of 0 means no limit.
func (b *Bin) HasWeightLimit() bool {
	return b.MaxWeight > 0
}

// RemainingWeight returns how much weight capacity is left, or +Inf if the
// bin has no weight limit.
func (b *Bin) RemainingWeight() float64 {
	if !b.HasWeightLimit() {
		return math.Inf(1)
	}
	return b.MaxWeight - b.weight
}

// AllowsWeight reports whether the bin can carry a total weight of w.
func (b *Bin) AllowsWeight(w float64) bool {
	return !b.HasWeightLimit() || w <= b.MaxWeight+weightTolerance
}

// CanCarry reports whether an item of weight w fits within the bin's
// remaining weight capacity.
func (b *Bin) CanCarry(w float64) bool {
	return b.AllowsWeight(b.weight + w)
}

// UsedVolume returns the sum of volumes of all placed items.
func (b *Bin) UsedVolume() float64 {
	return b.volume
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
	TotalCost     float64 // sum of Cost for all used bins (0 if costs not set)
}

// Result holds the outcome of a packing operation.
type Result struct {
	Bins          []*Bin
	UnfittedItems []*Item
	Stats         PackingStats
}
