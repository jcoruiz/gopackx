// Package stability provides physical stability checks for item placements
// in 3D bin packing.
//
// These functions evaluate whether a packed configuration is physically
// realistic by checking support surfaces, load-bearing limits, and weight
// distribution.
//
// # Support Ratio
//
// [SupportRatio] calculates the fraction (0.0 to 1.0) of an item's base area
// that is supported by items directly below it or the bin floor. Items resting
// on the floor always have a support ratio of 1.0.
//
//	ratio := stability.SupportRatio(topItem, allPlacedItems)
//	// ratio = 0.75 means 75% of the base is supported
//
// [CheckSupport] is a convenience that returns true if the support ratio meets
// or exceeds a given threshold.
//
// # Load Bearing
//
// [WeightAbove] calculates the total weight resting directly on top of an item,
// proportional to the overlap area between the item and items above it.
//
// [CheckLoadBearing] returns true if the weight above does not exceed the item's
// load-bearing capacity. Items with zero LoadBear and non-fragile status have
// no limit enforced. Fragile items must have zero weight above them.
//
// # Gravity Center
//
// [GravityCenter] analyzes weight distribution across four quadrants of the bin
// floor (Q1=front-left, Q2=front-right, Q3=back-left, Q4=back-right). It
// returns four percentages that sum to 100, indicating how weight is distributed.
// A perfectly balanced load returns [25, 25, 25, 25]. This is useful for
// detecting unbalanced loads that might cause tipping.
//
//	gc := stability.GravityCenter(bin.Items, bin.Width, bin.Depth)
//	// gc = [30.5, 19.5, 28.0, 22.0]
package stability
