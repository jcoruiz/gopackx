// Package placement provides item placement engines for 3D bin packing.
//
// A placement engine implements the [Engine] interface, which has a single
// method: PlaceItem(bin, item) bool. The engine determines where and in what
// orientation to place an item inside a bin. On success it sets the item's
// Position, RotationType, and Placed fields, and appends it to the bin's Items
// slice.
//
// # Engines
//
// Three placement engines are provided, each with different trade-offs:
//
// [PivotEngine] generates candidate positions from the corners of already-placed
// items (pivot points). It tries each pivot with every allowed rotation and
// selects the first valid placement. After placement, fix-point correction
// pushes the item toward the origin to eliminate floating gaps. This is the
// default engine used by the packer.
//
//	engine := placement.NewPivotEngine(
//	    placement.WithStability(0.7), // require 70% base support
//	)
//
// [ExtremePointEngine] maintains a scored list of candidate positions with
// metadata about available space and supporting surfaces. It evaluates all
// candidate points and rotations, selecting the placement with the best score
// (most support, lowest position, closest to origin, tightest fit). This engine
// generally produces denser packings than the PivotEngine at the cost of more
// computation.
//
//	engine := placement.NewExtremePointEngine(
//	    placement.WithEPStability(0.8),
//	)
//
// [LAFFEngine] (Largest Area Fit First) uses level-based horizontal stacking.
// It divides the bin into horizontal levels where the first item in each level
// defines the level height. Items are packed within levels, then levels are
// stacked vertically. The fast variant restricts placement to 2D within each
// level.
//
//	engine := placement.NewLAFFEngine(
//	    placement.WithLAFFStability(0.6),
//	    placement.LAFFFast(), // 2D-only within levels
//	)
//
// # Common Features
//
// All engines share the following capabilities:
//
//   - Fix-point correction: after initial placement, items are pushed toward
//     the origin along each axis to close gaps.
//   - Stability checking: optional minimum support ratio threshold. When enabled,
//     an item's base area must be sufficiently supported by items below or the floor.
//   - Fragile items: items marked as fragile will never have other items placed
//     on top of them.
//   - Weight limits: the bin's maximum weight capacity is enforced during placement.
//   - Load-bearing limits: when stability is enabled, placing an item must not
//     exceed the load-bearing capacity of items below it.
package placement
