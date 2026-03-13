package model

import "testing"

func TestNewItem(t *testing.T) {
	item := NewItem("test", 10, 20, 30, 5.0)

	if item.ID != "test" {
		t.Errorf("ID = %q, want %q", item.ID, "test")
	}
	if item.Volume != 6000 {
		t.Errorf("Volume = %f, want %f", item.Volume, 6000.0)
	}
	if len(item.AllowedRotations) != 6 {
		t.Errorf("AllowedRotations count = %d, want 6", len(item.AllowedRotations))
	}
}

func TestNewItemWithOptions(t *testing.T) {
	item := NewItem("fragile-box", 10, 20, 30, 5.0,
		ItemUpright(),
		ItemPriority(1),
		ItemLoadBear(10.0),
		ItemGroup("electronics"),
	)

	if len(item.AllowedRotations) != 2 {
		t.Errorf("AllowedRotations count = %d, want 2 (upright)", len(item.AllowedRotations))
	}
	if item.Priority != 1 {
		t.Errorf("Priority = %d, want 1", item.Priority)
	}
	if item.LoadBear != 10.0 {
		t.Errorf("LoadBear = %f, want 10.0", item.LoadBear)
	}
	if item.Group != "electronics" {
		t.Errorf("Group = %q, want %q", item.Group, "electronics")
	}
}

func TestItemFragile(t *testing.T) {
	item := NewItem("glass", 5, 5, 5, 1.0, ItemFragile())
	if !item.Fragile {
		t.Error("expected Fragile = true")
	}
}

func TestItemDimension(t *testing.T) {
	tests := []struct {
		name     string
		rotation RotationType
		want     [3]float64
	}{
		{"WHD", RotationWHD, [3]float64{10, 20, 30}},
		{"HWD", RotationHWD, [3]float64{20, 10, 30}},
		{"HDW", RotationHDW, [3]float64{20, 30, 10}},
		{"DHW", RotationDHW, [3]float64{30, 20, 10}},
		{"DWH", RotationDWH, [3]float64{30, 10, 20}},
		{"WDH", RotationWDH, [3]float64{10, 30, 20}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := NewItem("test", 10, 20, 30, 1.0)
			item.RotationType = tt.rotation
			got := item.Dimension()
			if got != tt.want {
				t.Errorf("Dimension() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBinWeightAndVolume(t *testing.T) {
	bin := NewBin("box", 10, 10, 10, 50.0)

	if bin.Volume != 1000 {
		t.Errorf("Volume = %f, want 1000", bin.Volume)
	}
	if bin.RemainingWeight() != 50.0 {
		t.Errorf("RemainingWeight = %f, want 50", bin.RemainingWeight())
	}

	item := NewItem("i1", 5, 5, 5, 10.0)
	bin.PlaceItem(item)

	if bin.TotalWeight() != 10.0 {
		t.Errorf("TotalWeight = %f, want 10", bin.TotalWeight())
	}
	if bin.RemainingWeight() != 40.0 {
		t.Errorf("RemainingWeight = %f, want 40", bin.RemainingWeight())
	}
	if bin.UsedVolume() != 125.0 {
		t.Errorf("UsedVolume = %f, want 125", bin.UsedVolume())
	}
	if bin.VolumeUsedPct() != 12.5 {
		t.Errorf("VolumeUsedPct = %f, want 12.5", bin.VolumeUsedPct())
	}
}
