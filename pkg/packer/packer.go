// Package packer provides the public API for GoPackX.
package packer

import (
	"context"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// Packer orchestrates 3D bin packing using a placement engine and strategy.
type Packer struct {
	bins     []*model.Bin
	items    []*model.Item
	strategy strategy.Type
	engine   placement.Engine
}

// Option configures the Packer.
type Option func(*Packer)

// WithStrategy sets the packing strategy.
func WithStrategy(st strategy.Type) Option {
	return func(p *Packer) { p.strategy = st }
}

// WithPlacementEngine sets a custom placement engine.
func WithPlacementEngine(e placement.Engine) Option {
	return func(p *Packer) { p.engine = e }
}

// NewPacker creates a new Packer with the given options.
// Defaults: BestFitDecreasing strategy, PivotEngine placement.
func NewPacker(opts ...Option) *Packer {
	p := &Packer{
		strategy: strategy.BestFitDecreasing,
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.engine == nil {
		p.engine = placement.NewPivotEngine()
	}
	return p
}

// AddBin adds a bin to the packer.
func (p *Packer) AddBin(bin *model.Bin) {
	p.bins = append(p.bins, bin)
}

// AddItem adds an item to be packed.
func (p *Packer) AddItem(item *model.Item) {
	p.items = append(p.items, item)
}

// Pack runs the packing algorithm and returns the result. It packs copies
// of the bins and items that were added, which stay unchanged, so calling
// Pack again gives the same result.
func (p *Packer) Pack(ctx context.Context) (*model.Result, error) {
	if err := model.Validate(p.bins, p.items); err != nil {
		return nil, err
	}
	// Pack copies: the bins and items that were added stay as they are, and
	// calling Pack again starts from them again.
	bins := make([]*model.Bin, len(p.bins))
	for i, b := range p.bins {
		bins[i] = b.Clone()
	}
	items := make([]*model.Item, len(p.items))
	for i, it := range p.items {
		items[i] = it.Clone()
		items[i].ResetPlacement()
	}
	if len(bins) == 0 || len(items) == 0 {
		return &model.Result{
			Bins:          bins,
			UnfittedItems: items,
			Stats:         computeStats(bins, items, items),
		}, nil
	}

	// Engines remember the bins they pack; start each run from scratch.
	if r, ok := p.engine.(placement.Resetter); ok {
		r.Reset()
	}

	strategy.SortItems(items, p.strategy)

	var unfitted []*model.Item

	if p.strategy == strategy.NextFit {
		unfitted = p.packNextFit(ctx, bins, items)
	} else {
		unfitted = p.packStandard(ctx, bins, items)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	result := &model.Result{
		Bins:          bins,
		UnfittedItems: unfitted,
		Stats:         computeStats(bins, items, unfitted),
	}
	return result, nil
}

func (p *Packer) packStandard(ctx context.Context, bins []*model.Bin, items []*model.Item) []*model.Item {
	var unfitted []*model.Item

	for _, item := range items {
		if ctx.Err() != nil {
			unfitted = append(unfitted, item)
			continue
		}

		placed := false
		candidates := strategy.SortBinsForItem(bins, item, p.strategy)
		for _, bin := range candidates {
			if p.engine.PlaceItem(bin, item) {
				placed = true
				break
			}
		}
		if !placed {
			unfitted = append(unfitted, item)
		}
	}
	return unfitted
}

func (p *Packer) packNextFit(ctx context.Context, bins []*model.Bin, items []*model.Item) []*model.Item {
	var unfitted []*model.Item
	binIdx := 0

	for _, item := range items {
		if ctx.Err() != nil {
			unfitted = append(unfitted, item)
			continue
		}

		placed := false
		for binIdx < len(bins) {
			if p.engine.PlaceItem(bins[binIdx], item) {
				placed = true
				break
			}
			binIdx++
		}
		if !placed {
			unfitted = append(unfitted, item)
		}
	}
	return unfitted
}

func computeStats(bins []*model.Bin, allItems, unfitted []*model.Item) model.PackingStats {
	activeBins := 0
	totalVolPct := 0.0
	totalWeight := 0.0
	totalMaxWeight := 0.0
	totalCost := 0.0

	for _, bin := range bins {
		if len(bin.Items) > 0 {
			activeBins++
			totalVolPct += bin.VolumeUsedPct()
			totalWeight += bin.TotalWeight()
			totalMaxWeight += bin.MaxWeight
			totalCost += bin.Cost
		}
	}

	avgVolPct := 0.0
	avgWeightPct := 0.0
	if activeBins > 0 {
		avgVolPct = totalVolPct / float64(activeBins)
		if totalMaxWeight > 0 {
			avgWeightPct = totalWeight / totalMaxWeight * 100
		}
	}

	return model.PackingStats{
		TotalBins:     activeBins,
		TotalItems:    len(allItems),
		FittedItems:   len(allItems) - len(unfitted),
		UnfittedCount: len(unfitted),
		VolumeUsedPct: avgVolPct,
		WeightUsedPct: avgWeightPct,
		TotalCost:     totalCost,
	}
}
