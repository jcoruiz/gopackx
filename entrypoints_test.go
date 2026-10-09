package gopackx_test

import (
	"context"
	"errors"
	"math"
	"testing"

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
