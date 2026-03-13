package stability

import (
	"math"
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestSupportRatio(t *testing.T) {
	tests := []struct {
		name   string
		item   *model.Item
		placed []*model.Item
		want   float64
	}{
		{
			name: "on floor",
			item: itemAt(0, 0, 0, 10, 10, 10, 1),
			want: 1.0,
		},
		{
			name: "fully supported by one item",
			item: itemAt(0, 10, 0, 10, 10, 10, 1),
			placed: []*model.Item{
				itemAt(0, 0, 0, 10, 10, 10, 1),
			},
			want: 1.0,
		},
		{
			name: "half supported",
			item: itemAt(0, 10, 0, 10, 10, 10, 1),
			placed: []*model.Item{
				itemAt(0, 0, 0, 5, 10, 10, 1),
			},
			want: 0.5,
		},
		{
			name: "no support (floating)",
			item: itemAt(0, 20, 0, 10, 10, 10, 1),
			placed: []*model.Item{
				itemAt(0, 0, 0, 10, 10, 10, 1),
			},
			want: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SupportRatio(tt.item, tt.placed)
			if math.Abs(got-tt.want) > 0.01 {
				t.Errorf("SupportRatio() = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestCheckSupport(t *testing.T) {
	floor := itemAt(0, 0, 0, 10, 10, 10, 1)
	above := itemAt(0, 10, 0, 10, 10, 10, 1)
	placed := []*model.Item{floor}

	if !CheckSupport(above, placed, 0.75) {
		t.Error("expected item fully supported to pass 0.75 ratio check")
	}

	halfSupport := itemAt(0, 10, 0, 20, 10, 10, 1)
	if CheckSupport(halfSupport, placed, 0.75) {
		t.Error("expected half-supported item to fail 0.75 ratio check")
	}
}

func TestWeightAbove(t *testing.T) {
	base := itemAt(0, 0, 0, 10, 10, 10, 5)
	top := itemAt(0, 10, 0, 10, 10, 10, 3)
	placed := []*model.Item{base, top}

	got := WeightAbove(base, placed)
	if math.Abs(got-3.0) > 0.01 {
		t.Errorf("WeightAbove() = %f, want 3.0", got)
	}
}

func TestWeightAbovePartial(t *testing.T) {
	base := itemAt(0, 0, 0, 10, 10, 10, 5)
	// Half overlapping on top.
	top := itemAt(5, 10, 0, 10, 10, 10, 4)
	placed := []*model.Item{base, top}

	got := WeightAbove(base, placed)
	// Overlap: 5x10 out of 10x10 = 50% of top's base area.
	want := 4.0 * 0.5
	if math.Abs(got-want) > 0.01 {
		t.Errorf("WeightAbove() = %f, want %f", got, want)
	}
}

func TestCheckLoadBearing(t *testing.T) {
	base := itemAt(0, 0, 0, 10, 10, 10, 5)
	base.LoadBear = 5.0
	light := itemAt(0, 10, 0, 10, 10, 10, 3)
	heavy := itemAt(0, 10, 0, 10, 10, 10, 8)

	if !CheckLoadBearing(base, []*model.Item{base, light}) {
		t.Error("3kg on 5kg capacity should pass")
	}
	if CheckLoadBearing(base, []*model.Item{base, heavy}) {
		t.Error("8kg on 5kg capacity should fail")
	}
}

func TestCheckLoadBearingFragile(t *testing.T) {
	fragile := itemAt(0, 0, 0, 10, 10, 10, 2)
	fragile.Fragile = true
	top := itemAt(0, 10, 0, 10, 10, 10, 1)

	if CheckLoadBearing(fragile, []*model.Item{fragile, top}) {
		t.Error("fragile item should not bear any weight")
	}
	if !CheckLoadBearing(fragile, []*model.Item{fragile}) {
		t.Error("fragile item with nothing on top should pass")
	}
}

func TestGravityCenter(t *testing.T) {
	t.Run("empty bin", func(t *testing.T) {
		got := GravityCenter(nil, 100, 100)
		for i, v := range got {
			if math.Abs(v-25) > 0.01 {
				t.Errorf("Q%d = %f, want 25", i+1, v)
			}
		}
	})

	t.Run("centered item", func(t *testing.T) {
		item := itemAt(25, 0, 25, 50, 10, 50, 10)
		got := GravityCenter([]*model.Item{item}, 100, 100)
		for i, v := range got {
			if math.Abs(v-25) > 0.01 {
				t.Errorf("Q%d = %f, want 25", i+1, v)
			}
		}
	})

	t.Run("item in Q1 only", func(t *testing.T) {
		item := itemAt(0, 0, 0, 10, 10, 10, 10)
		got := GravityCenter([]*model.Item{item}, 100, 100)
		if got[0] != 100 {
			t.Errorf("Q1 = %f, want 100", got[0])
		}
		for i := 1; i < 4; i++ {
			if got[i] != 0 {
				t.Errorf("Q%d = %f, want 0", i+1, got[i])
			}
		}
	})
}

func itemAt(x, y, z, w, h, d, weight float64) *model.Item {
	item := model.NewItem("", w, h, d, weight)
	item.Position = [3]float64{x, y, z}
	return item
}
