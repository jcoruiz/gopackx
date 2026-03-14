// Package model defines the core data types for 3D bin packing.
//
// This package provides the foundational types used throughout GoPackX:
// items to be packed, bins (containers) to pack them into, rotation types
// for orienting items, and result types that capture packing outcomes.
//
// # Items
//
// An [Item] represents a physical object with width, height, depth, and weight.
// Items are created with [NewItem] and configured using the functional options pattern:
//
//	item := model.NewItem("box-1", 10, 20, 15, 5.0,
//	    model.ItemUpright(),
//	    model.ItemPriority(1),
//	    model.ItemLoadBear(50),
//	)
//
// Available item options:
//   - [ItemUpright]: restricts rotations to keep the height axis vertical (2 rotations instead of 6).
//   - [ItemFragile]: marks the item so nothing can be placed on top of it.
//   - [ItemLoadBear]: sets the maximum weight the item can support on top (kg).
//   - [ItemPriority]: sets packing priority (1 = highest priority, packed first).
//   - [ItemGroup]: assigns the item to a binding group so grouped items stay together.
//   - [ItemAllowedRotations]: provides a custom set of allowed rotation types.
//
// After packing, each placed item has its [Item.Position] (origin corner in [x, y, z]),
// [Item.RotationType], and [Item.Placed] fields set by the placement engine.
//
// The [Item.Dimension] method returns the effective [width, height, depth] after
// applying the item's current rotation. This accounts for the permutation defined
// by the active [RotationType].
//
// # Rotation Types
//
// There are 6 possible orientations for a rectangular item, represented by
// [RotationType] constants (RotationWHD through RotationWDH). Each rotation
// permutes which physical dimension maps to width, height, and depth. Use
// [AllRotations] and [UprightRotations] to obtain the canonical rotation lists.
//
// # Bins
//
// A [Bin] represents a container with fixed width, height, depth, and maximum
// weight capacity. Create bins with [NewBin]:
//
//	bin := model.NewBin("container-1", 100, 100, 100, 500)
//
// After packing, [Bin.Items] holds the successfully placed items and helper methods
// such as [Bin.TotalWeight], [Bin.RemainingWeight], [Bin.UsedVolume], and
// [Bin.VolumeUsedPct] provide utilization metrics.
//
// # Results
//
// A [Result] aggregates the packing outcome: the list of bins (with their placed
// items), any items that could not be fitted, and summary [PackingStats] including
// total bins used, fitted/unfitted counts, and average volume and weight utilization.
package model
