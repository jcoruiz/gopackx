package model

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	okBin := func() *Bin { return NewBin("box", 10, 10, 10, 50) }
	okItem := func() *Item { return NewItem("item", 1, 2, 3, 4) }

	tests := []struct {
		name  string
		bins  []*Bin
		items []*Item
		want  string // substring of the error; empty means valid
	}{
		{"valid", []*Bin{okBin()}, []*Item{okItem()}, ""},
		{"empty input is valid", nil, nil, ""},
		{"no weight limit", []*Bin{NewBin("box", 10, 10, 10, 0)}, nil, ""},
		{"nil bin", []*Bin{nil}, nil, `bin "" (index 0): is nil`},
		{"nil item", nil, []*Item{okItem(), nil}, `item "" (index 1): is nil`},
		{"zero width", []*Bin{NewBin("box", 0, 10, 10, 50)}, nil, "Width must be a finite number greater than 0, got 0"},
		{"negative height", nil, []*Item{NewItem("a", 1, -2, 3, 4)}, `item "a" (index 0): Height must be`},
		{"NaN depth", nil, []*Item{NewItem("a", 1, 2, math.NaN(), 4)}, "Depth must be a finite number greater than 0, got NaN"},
		{"infinite bin", []*Bin{NewBin("box", math.Inf(1), 10, 10, 50)}, nil, "got +Inf"},
		{"negative weight", nil, []*Item{NewItem("a", 1, 2, 3, -50)}, "Weight must be a finite number not below 0, got -50"},
		{"NaN weight limit", []*Bin{NewBin("box", 10, 10, 10, math.NaN())}, nil, "MaxWeight must be"},
		{"negative cost", []*Bin{NewBin("box", 10, 10, 10, 5, BinCost(-1))}, nil, "Cost must be"},
		{"negative load limit", nil, []*Item{NewItem("a", 1, 2, 3, 4, ItemLoadBear(-1))}, "LoadBear must be"},
		{"no rotations", nil, []*Item{NewItem("a", 1, 2, 3, 4, ItemAllowedRotations(nil))}, "AllowedRotations is empty"},
		{"unknown rotation", nil, []*Item{NewItem("a", 1, 2, 3, 4, ItemAllowedRotations([]RotationType{7}))}, "unknown rotation 7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.bins, tt.items)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("Validate() = nil, want an error containing %q", tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() = %q, want it to contain %q", err, tt.want)
			}
			if !errors.Is(err, ErrInvalidInput) {
				t.Error("error does not wrap ErrInvalidInput")
			}
			var ve *ValidationError
			if !errors.As(err, &ve) {
				t.Error("error is not a *ValidationError")
			}
		})
	}
}
