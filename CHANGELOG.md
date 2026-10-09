# Changelog

## v0.3.0 (2026-10-09)

This release fixes physical rules that packings could break, makes results deterministic and consistent across solvers, and speeds up the stateful engines. Several behaviors change: read **Breaking changes** before upgrading.

Fuzzing every solver and engine against an independent checker found the bugs below. On the same random scenarios, v0.2.2 broke a rule in thousands of runs; v0.3.0 breaks none.

### Breaking changes

- **Load limits count the whole stack and always apply.** `LoadBear` used to count only the items touching an item, and only with stability enabled. A crate that holds 30 kg could carry 15 kg on top plus 40 kg stacked on that. Now the load on an item is everything stacked on it, split among supports by contact area, and limits apply with or without stability. Packings with load-limited items may use more boxes; they are physically valid now.
- **A `MaxWeight` of 0 means no weight limit**, like a `Cost` or `LoadBear` of 0. It used to mean that the bin could hold nothing, silently leaving every item unfitted.
- **Invalid input returns an error.** `gopackx.Pack`, every `Solve` and `Packer.Pack` call `model.Validate` and return a nil result and a `*model.ValidationError` (wrapping `model.ErrInvalidInput`) for NaN or non-positive sizes, negative weights, limits or costs, and empty or unknown rotations. Before, such input was packed silently: a NaN-sized item was "placed", and a negative weight let a 1 kg box take items of -50 and 40 kg.
- **Context errors are reported consistently.** When the context ends first, every solver and `Packer.Pack` return the best result so far **and** the context error. `Metaheuristic`, `BranchBound` and `Parallel` used to return a nil error (so "cancelled" looked like "nothing fits"), and `Packer.Pack` returned a nil result.
- **`Packer.Pack` packs copies.** The bins passed to `AddBin` are no longer filled, and the items passed to `AddItem` are no longer modified: read `Result.Bins`. Calling `Pack` twice gives the same result (it used to append the items again).
- **Bin internals are no longer exported.** `Bin.AABBData`, `Bin.HasFragile`, `Bin.FragileIdxs`, `Bin.ItemWeight` and `Bin.ItemVolume` are private, `Item.PlacedDim` and the never-filled `Bin.UnfittedItems` are gone. Use `TotalWeight`, `UsedVolume`, `Box`, `Load` and the other queries. Change a bin's items only with `PlaceItem` and `RemoveLastItem`.
- `stability.CheckLoadBearing` uses the full stacked load (`stability.LoadOnTop`), and treats any item touching a fragile item as a violation. `stability.WeightAbove` keeps counting only direct contact.

### Fixed

- **Fragile and load-limited items could be slid under items placed earlier**, for example under an overhang, ending up with weight on them. Placement only checked one direction. Found in 554 of 15,740 fuzz runs, with every engine and solver.
- **An overhanging item's weight partly vanished**: each support received `contact / own base` of it, so at 60% support 40% of the weight went nowhere. The whole weight now goes to the supports.
- **MaxRects lost free space**: splitting several free spaces for one item dropped the sub-spaces of every split but the last. A pallet that holds 400 items received 79. It also gave up when its best-scoring spot failed validation instead of trying the next, and lifted items in a gap onto the items above the gap. Over 400 random scenarios MaxRects now uses 20% fewer boxes (fill 39.8% -> 46.9%), on par with Pivot and Extreme Points.
- **The Metaheuristic was not deterministic**: it used the global random source, so the same input gave a different packing on 13 of 40 scenarios, some with a lower fill. It also matched box types by size only, reporting the expensive one of two same-size types with different costs.
- **Parallel picked whichever tied configuration finished first.** Ties now go to the first configuration.
- **Deadlines were ignored in WebAssembly** (`GOOS=js`), where the timer behind `context.WithTimeout` cannot fire while a solver computes. Branch & Bound and TrialPacking now also stop within one placement of the deadline (Branch & Bound took 58 ms for a 30 ms deadline).
- **Results depended on how often a solver switched bins**: Extreme Points and LAFF rebuilt their state differently from how they had built it while packing.
- **Engines ignored items removed with `RemoveLastItem`**: Extreme Points and MaxRects kept seeing the removed item and refused to place another one there. Engines now rebuild their state whenever the bin's revision changed.

### Performance

- Extreme Points, MaxRects and LAFF keep their state per bin instead of rebuilding it whenever a solver tries another open bin (7,081 rebuilds for 300 items). With TrialPacking and 800 items, Extreme Points takes 2 s instead of 9 minutes (Apple M4 Pro); with many small boxes, 32 ms instead of 1 s.
- Extreme Points scores candidates before validating them and finds duplicate points in O(1).
- 50 item benchmark (AMD Ryzen 9 9950X3D, Go 1.22): Extreme Points 4.3 -> 1.4 ms, LAFF 0.09 -> 0.08 ms, Parallel 4.6 -> 1.8 ms. Pivot 0.08 -> 0.10 ms and LAFF-Fast 0.06 -> 0.08 ms are slightly slower: placement checks both directions for fragile and load-limited items, and the Packer packs copies.
- Bins only track loads once an item with a load limit is involved.

### Added

- `model.Validate`, `model.ValidationError`, `model.ErrInvalidInput`.
- `model.Bin`: `TypeID` (the box type a solver opened the bin from), `Revision`, `Load`, `FitsLoadLimits`, `Box`, `Collides`, `RestsOnFragile`, `HasItemOnTop`, `SlideToOrigin`, `HasWeightLimit`, `AllowsWeight`, `CanCarry`, `Clone`, `CloneEmpty`.
- `model.Item`: `Clone`, `ResetPlacement`.
- `solver.MetaRandomSeed` to explore other metaheuristic solutions.
- `stability.LoadOnTop`.
- `placement.Resetter`; every engine has `Reset` to forget its per-bin state before an unrelated run.
- Property and fuzz tests (`FuzzPhysicalRules`) that check every solver and engine against the physical rules with code independent of the library, and a CI job that runs the suite as WebAssembly.
