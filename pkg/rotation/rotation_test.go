package rotation

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestDimensions(t *testing.T) {
	tests := []struct {
		name string
		rt   model.RotationType
		want [3]float64
	}{
		{"WHD", model.RotationWHD, [3]float64{10, 20, 30}},
		{"HWD", model.RotationHWD, [3]float64{20, 10, 30}},
		{"HDW", model.RotationHDW, [3]float64{20, 30, 10}},
		{"DHW", model.RotationDHW, [3]float64{30, 20, 10}},
		{"DWH", model.RotationDWH, [3]float64{30, 10, 20}},
		{"WDH", model.RotationWDH, [3]float64{10, 30, 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Dimensions(10, 20, 30, tt.rt)
			if got != tt.want {
				t.Errorf("Dimensions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDimensionsFor(t *testing.T) {
	item := model.NewItem("test", 10, 20, 30, 1.0)
	item.RotationType = model.RotationWHD

	got := DimensionsFor(item, model.RotationHWD)
	want := [3]float64{20, 10, 30}
	if got != want {
		t.Errorf("DimensionsFor(HWD) = %v, want %v", got, want)
	}
	// Original rotation unchanged.
	if item.RotationType != model.RotationWHD {
		t.Errorf("item.RotationType changed to %d", item.RotationType)
	}
}

func TestAllowedFor(t *testing.T) {
	t.Run("default all rotations", func(t *testing.T) {
		item := model.NewItem("test", 1, 2, 3, 1.0)
		got := AllowedFor(item)
		if len(got) != 6 {
			t.Errorf("AllowedFor = %d rotations, want 6", len(got))
		}
	})

	t.Run("upright only", func(t *testing.T) {
		item := model.NewItem("test", 1, 2, 3, 1.0, model.ItemUpright())
		got := AllowedFor(item)
		if len(got) != 2 {
			t.Errorf("AllowedFor = %d rotations, want 2", len(got))
		}
	})
}

func TestIsAllowed(t *testing.T) {
	item := model.NewItem("test", 1, 2, 3, 1.0, model.ItemUpright())

	if !IsAllowed(item, model.RotationWHD) {
		t.Error("WHD should be allowed for upright")
	}
	if !IsAllowed(item, model.RotationDHW) {
		t.Error("DHW should be allowed for upright")
	}
	if IsAllowed(item, model.RotationHWD) {
		t.Error("HWD should NOT be allowed for upright")
	}
}
