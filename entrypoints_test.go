package gopackx_test

import (
	"context"
	"errors"
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
	"github.com/jcoruiz/gopackx/pkg/packer"
	"github.com/jcoruiz/gopackx/pkg/placement"
	"github.com/jcoruiz/gopackx/pkg/solver"
)

type entryPoint struct {
	name string
	run  func(ctx context.Context, bins []*model.Bin, items []*model.Item) (*model.Result, error)
}

func newPivot() placement.Engine { return placement.NewPivotEngine() }

// entryPoints lists every public way to pack.
var entryPoints = []entryPoint{
	{"Pack", func(ctx context.Context, b []*model.Bin, i []*model.Item) (*model.Result, error) {
		return gopackx.Pack(ctx, b, i)
	}},
	{"Pack+Optimize", func(ctx context.Context, b []*model.Bin, i []*model.Item) (*model.Result, error) {
		return gopackx.Pack(ctx, b, i, gopackx.Optimize())
	}},
	{"TrialPacking", solver.NewTrialPacking(newPivot, solver.WithLookahead()).Solve},
	{"Metaheuristic", solver.NewMetaheuristic(newPivot).Solve},
	{"BranchBound", solver.NewBranchBound(newPivot).Solve},
	{"Parallel", solver.NewParallel().Solve},
	{"Packer", func(ctx context.Context, b []*model.Bin, i []*model.Item) (*model.Result, error) {
		p := packer.NewPacker()
		for _, x := range b {
			p.AddBin(x)
		}
		for _, x := range i {
			p.AddItem(x)
		}
		return p.Pack(ctx)
	}},
}

func TestEntryPointsRejectInvalidInput(t *testing.T) {
	cases := map[string]func() ([]*model.Bin, []*model.Item){
		"NaN item width": func() ([]*model.Bin, []*model.Item) {
			return []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, []*model.Item{model.NewItem("a", math.NaN(), 1, 1, 1)}
		},
		"NaN bin": func() ([]*model.Bin, []*model.Item) {
			return []*model.Bin{model.NewBin("b", math.NaN(), 10, 10, 100)}, []*model.Item{model.NewItem("a", 2, 2, 2, 1)}
		},
		// A negative weight used to offset a heavy item: a 1 kg box took
		// items of -50 and 40 kg.
		"negative weight": func() ([]*model.Bin, []*model.Item) {
			return []*model.Bin{model.NewBin("b", 10, 10, 10, 1)}, []*model.Item{model.NewItem("a", 2, 2, 2, -50), model.NewItem("c", 2, 2, 2, 40)}
		},
		"zero-size item": func() ([]*model.Bin, []*model.Item) {
			return []*model.Bin{model.NewBin("b", 10, 10, 10, 100)}, []*model.Item{model.NewItem("a", 0, 0, 0, 1)}
		},
	}
	for _, ep := range entryPoints {
		for name, mk := range cases {
			t.Run(ep.name+"/"+name, func(t *testing.T) {
				bins, items := mk()
				res, err := ep.run(context.Background(), bins, items)
				if !errors.Is(err, model.ErrInvalidInput) {
					t.Fatalf("err = %v, want one wrapping model.ErrInvalidInput", err)
				}
				if res != nil {
					t.Error("result should be nil for invalid input")
				}
			})
		}
	}
}

func heavyInput() ([]*model.Bin, []*model.Item) {
	bins := make([]*model.Bin, 40)
	for i := range bins {
		bins[i] = model.NewBin("pallet-"+strconv.Itoa(i), 80, 60, 60, 1e9)
	}
	items := make([]*model.Item, 1200)
	for i := range items {
		items[i] = model.NewItem("i"+strconv.Itoa(i), float64(5+i*7%23), float64(4+i*5%19), float64(6+i*11%25), 1)
	}
	return bins, items
}

// When the context is already cancelled, every entry point returns the
// (empty) best result together with the context error.
func TestEntryPointsReturnResultAndErrorWhenCancelled(t *testing.T) {
	for _, ep := range entryPoints {
		t.Run(ep.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			bins, items := heavyInput()
			res, err := ep.run(ctx, bins, items[:50])
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("err = %v, want context.Canceled", err)
			}
			if res == nil {
				t.Fatal("result is nil")
			}
			if res.Stats.FittedItems+res.Stats.UnfittedCount != 50 || res.Stats.TotalItems != 50 {
				t.Errorf("stats %+v do not account for the 50 items", res.Stats)
			}
		})
	}
}

// Every entry point stops soon after its deadline and says so. Branch &
// Bound used to finish the permutation in progress, overshooting a 30 ms
// deadline to 58 ms on this input.
func TestEntryPointsHonorDeadlines(t *testing.T) {
	const budget = 50 * time.Millisecond
	for _, ep := range entryPoints {
		t.Run(ep.name, func(t *testing.T) {
			bins, items := heavyInput()
			ctx, cancel := context.WithTimeout(context.Background(), budget)
			defer cancel()
			start := time.Now()
			res, err := ep.run(ctx, bins, items)
			took := time.Since(start)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want context.DeadlineExceeded", err)
			}
			if res == nil || res.Stats.FittedItems+res.Stats.UnfittedCount != len(items) {
				t.Fatalf("result does not account for every item: %+v", res)
			}
			if took > budget+100*time.Millisecond {
				t.Errorf("returned after %v, deadline was %v", took, budget)
			}
		})
	}
}

// pastDeadline has a deadline that passed but was never signalled, as with
// GOOS=js where the deadline timer cannot fire while a solver computes.
type pastDeadline struct{ context.Context }

func (pastDeadline) Deadline() (time.Time, bool) { return time.Now().Add(-time.Second), true }

func TestEntryPointsStopWhenTheDeadlineTimerCannotFire(t *testing.T) {
	for _, ep := range entryPoints {
		t.Run(ep.name, func(t *testing.T) {
			bins, items := heavyInput()
			res, err := ep.run(pastDeadline{context.Background()}, bins, items)
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want context.DeadlineExceeded", err)
			}
			if res == nil {
				t.Fatal("result is nil")
			}
		})
	}
}
