package intersection

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestRectIntersect(t *testing.T) {
	tests := []struct {
		name         string
		i1           *model.Item
		i2           *model.Item
		axis1, axis2 model.Axis
		want         bool
	}{
		{
			name:  "overlapping on WH",
			i1:    itemAt(0, 0, 0, 10, 10, 10),
			i2:    itemAt(5, 5, 0, 10, 10, 10),
			axis1: model.WidthAxis, axis2: model.HeightAxis,
			want: true,
		},
		{
			name:  "touching not overlapping",
			i1:    itemAt(0, 0, 0, 10, 10, 10),
			i2:    itemAt(10, 0, 0, 10, 10, 10),
			axis1: model.WidthAxis, axis2: model.HeightAxis,
			want: false,
		},
		{
			name:  "separated",
			i1:    itemAt(0, 0, 0, 5, 5, 5),
			i2:    itemAt(20, 20, 0, 5, 5, 5),
			axis1: model.WidthAxis, axis2: model.HeightAxis,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RectIntersect(tt.i1, tt.i2, tt.axis1, tt.axis2)
			if got != tt.want {
				t.Errorf("RectIntersect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIntersect(t *testing.T) {
	tests := []struct {
		name string
		i1   *model.Item
		i2   *model.Item
		want bool
	}{
		{
			name: "fully overlapping",
			i1:   itemAt(0, 0, 0, 10, 10, 10),
			i2:   itemAt(0, 0, 0, 10, 10, 10),
			want: true,
		},
		{
			name: "partial overlap all planes",
			i1:   itemAt(0, 0, 0, 10, 10, 10),
			i2:   itemAt(5, 5, 5, 10, 10, 10),
			want: true,
		},
		{
			name: "touching on X axis",
			i1:   itemAt(0, 0, 0, 10, 10, 10),
			i2:   itemAt(10, 0, 0, 10, 10, 10),
			want: false,
		},
		{
			name: "touching on Y axis",
			i1:   itemAt(0, 0, 0, 10, 10, 10),
			i2:   itemAt(0, 10, 0, 10, 10, 10),
			want: false,
		},
		{
			name: "touching on Z axis",
			i1:   itemAt(0, 0, 0, 10, 10, 10),
			i2:   itemAt(0, 0, 10, 10, 10, 10),
			want: false,
		},
		{
			name: "separated in all axes",
			i1:   itemAt(0, 0, 0, 5, 5, 5),
			i2:   itemAt(20, 20, 20, 5, 5, 5),
			want: false,
		},
		{
			name: "overlap WH only, separated on D",
			i1:   itemAt(0, 0, 0, 10, 10, 10),
			i2:   itemAt(5, 5, 20, 10, 10, 10),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Intersect(tt.i1, tt.i2)
			if got != tt.want {
				t.Errorf("Intersect() = %v, want %v", got, tt.want)
			}
		})
	}
}

func itemAt(x, y, z, w, h, d float64) *model.Item {
	item := model.NewItem("", w, h, d, 1.0)
	item.Position = [3]float64{x, y, z}
	return item
}
