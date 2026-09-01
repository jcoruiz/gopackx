package solver

import (
	"context"
	"sync"

	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/strategy"
)

// Verify interface compliance.
var _ Solver = (*Parallel)(nil)

// Parallel runs multiple packing configurations concurrently and returns the best result.
// Each configuration gets its own deep copy of bins and items to work with independently.
type Parallel struct {
	configs []ParallelConfig
}

// ParallelConfig defines a single packing configuration to run.
type ParallelConfig struct {
	NewEngine func() placement.Engine
	Strategy  strategy.Type
}

// ParallelOption configures the Parallel solver.
type ParallelOption func(*Parallel)

// WithConfig adds a packing configuration to try.
func WithConfig(newEngine func() placement.Engine, st strategy.Type) ParallelOption {
	return func(p *Parallel) {
		p.configs = append(p.configs, ParallelConfig{NewEngine: newEngine, Strategy: st})
	}
}

// NewParallel creates a new Parallel solver.
// If no configs are provided, uses a default set covering all engines and key strategies.
func NewParallel(opts ...ParallelOption) *Parallel {
	p := &Parallel{}
	for _, opt := range opts {
		opt(p)
	}
	if len(p.configs) == 0 {
		p.configs = defaultConfigs()
	}
	return p
}

func defaultConfigs() []ParallelConfig {
	return []ParallelConfig{
		{func() placement.Engine { return placement.NewPivotEngine() }, strategy.BestFitDecreasing},
		{func() placement.Engine { return placement.NewPivotEngine() }, strategy.MinimizeBins},
		{func() placement.Engine { return placement.NewExtremePointEngine() }, strategy.BestFitDecreasing},
		{func() placement.Engine { return placement.NewLAFFEngine() }, strategy.BestFitDecreasing},
		{func() placement.Engine { return placement.NewLAFFEngine(placement.LAFFFast()) }, strategy.BestFitDecreasing},
	}
}

// Solve runs all configurations concurrently and returns the best result.
func (p *Parallel) Solve(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error) {
	if len(p.configs) == 0 || len(bins) == 0 || len(items) == 0 {
		return &model.Result{
			Bins:          bins,
			UnfittedItems: items,
			Stats:         computeStats(bins, items, items),
		}, nil
	}

	type entry struct {
		res *model.Result
	}

	results := make(chan entry, len(p.configs))
	var wg sync.WaitGroup

	for _, cfg := range p.configs {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Deep copy bins and items for this goroutine.
			binsCopy := make([]*model.Bin, len(bins))
			for i, b := range bins {
				binsCopy[i] = cloneBinEmpty(b)
			}
			itemsCopy := resetItems(items)

			engine := cfg.NewEngine()
			res := packGreedy(ctx, engine, binsCopy, itemsCopy, cfg.Strategy)
			results <- entry{res}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var best *model.Result
	for r := range results {
		if isBetter(r.res, best) {
			best = r.res
		}
	}

	if best == nil {
		return &model.Result{
			Bins:          bins,
			UnfittedItems: items,
			Stats:         computeStats(bins, items, items),
		}, ctx.Err()
	}

	return best, nil
}

func isBetter(a, b *model.Result) bool {
	if b == nil {
		return true
	}
	if a.Stats.FittedItems != b.Stats.FittedItems {
		return a.Stats.FittedItems > b.Stats.FittedItems
	}
	return a.Stats.VolumeUsedPct > b.Stats.VolumeUsedPct
}

// packGreedy packs items into bins using the given engine and strategy.
func packGreedy(ctx context.Context, engine placement.Engine, bins []*model.Bin, items []*model.Item, st strategy.Type) *model.Result {
	strategy.SortItems(items, st)

	var unfitted []*model.Item
	for _, item := range items {
		if ctx.Err() != nil {
			unfitted = append(unfitted, item)
			continue
		}

		placed := false
		candidates := strategy.SortBinsForItem(bins, item, st)
		for _, bin := range candidates {
			if engine.PlaceItem(bin, item) {
				placed = true
				break
			}
		}
		if !placed {
			unfitted = append(unfitted, item)
		}
	}

	return &model.Result{
		Bins:          bins,
		UnfittedItems: unfitted,
		Stats:         computeStats(bins, items, unfitted),
	}
}
