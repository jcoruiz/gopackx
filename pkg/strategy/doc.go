// Package strategy provides item ordering and bin selection strategies for
// 3D bin packing.
//
// A strategy controls two aspects of the packing process:
//
//  1. Item sort order: how items are ordered before placement (by volume
//     ascending or descending, with secondary criteria for load-bearing
//     capacity, priority, and group).
//  2. Bin selection: how candidate bins are ranked for each item (first fit,
//     best fit, worst fit, etc.).
//
// # Available Strategies
//
// Seven strategies are defined as [Type] constants:
//
//   - [MinimizeBins]: sorts items by volume descending, uses first-fit bin
//     selection. Tends to fill bins fully before moving to the next.
//   - [Greedy]: sorts items by volume ascending, uses first-fit. Places
//     smallest items first.
//   - [BestFit]: sorts items by volume ascending, selects the bin with the
//     smallest remaining volume (tightest fit).
//   - [BestFitDecreasing]: sorts items by volume descending, selects the bin
//     with the smallest remaining volume. This is the default strategy and
//     generally produces good results across a wide range of inputs.
//   - [NextFit]: sorts items by volume ascending, only tries the current bin
//     (never goes back to earlier bins). Fastest but lowest quality.
//   - [WorstFit]: sorts items by volume ascending, selects the bin with the
//     largest remaining volume (most space).
//   - [AlmostWorstFit]: like WorstFit but skips bins that are more than 80%
//     empty, preferring partially-filled bins.
//
// # Item Sorting
//
// [SortItems] applies a multi-criteria stable sort:
//  1. Volume (direction depends on strategy)
//  2. Load-bearing capacity descending (sturdy items first)
//  3. Priority ascending (1 = highest priority)
//  4. Group lexicographic (keeps binding groups together)
//
// # Bin Selection
//
// [SortBinsForItem] returns candidate bins filtered by weight capacity and
// ordered according to the strategy. NextFit bin selection is handled by the
// packer itself, not by this function.
package strategy
