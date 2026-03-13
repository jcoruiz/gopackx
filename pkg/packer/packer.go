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

// Pack runs the packing algorithm and returns the result.
func (p *Packer) Pack(ctx context.Context) (*model.Result, error) {
	if len(p.bins) == 0 || len(p.items) == 0 {
		return &model.Result{
			Bins:          p.bins,
			UnfittedItems: p.items,
			Stats:         computeStats(p.bins, p.items, p.items),
		}, nil
	}

	// Copy the item slice for sorting (items themselves are shared).
	items := make([]*model.Item, len(p.items))
	copy(items, p.items)
	strategy.SortItems(items, p.strategy)

	var unfitted []*model.Item

	if p.strategy == strategy.NextFit {
		unfitted = p.packNextFit(ctx, items)
	} else {
		unfitted = p.packStandard(ctx, items)
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	result := &model.Result{
		Bins:          p.bins,
		UnfittedItems: unfitted,
		Stats:         computeStats(p.bins, items, unfitted),
	}
	return result, nil
}

func (p *Packer) packStandard(ctx context.Context, items []*model.Item) []*model.Item {
	var unfitted []*model.Item

	for _, item := range items {
		if ctx.Err() != nil {
			unfitted = append(unfitted, item)
			continue
		}

		placed := false
		candidates := strategy.SortBinsForItem(p.bins, item, p.strategy)
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

func (p *Packer) packNextFit(ctx context.Context, items []*model.Item) []*model.Item {
	var unfitted []*model.Item
	binIdx := 0

	for _, item := range items {
		if ctx.Err() != nil {
			unfitted = append(unfitted, item)
			continue
		}

		placed := false
		for binIdx < len(p.bins) {
			if p.engine.PlaceItem(p.bins[binIdx], item) {
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

	for _, bin := range bins {
		if len(bin.Items) > 0 {
			activeBins++
			totalVolPct += bin.VolumeUsedPct()
			totalWeight += bin.TotalWeight()
			totalMaxWeight += bin.MaxWeight
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
	}
}
