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

func TestItemAllowedRotations(t *testing.T) {
	rots := []RotationType{RotationWHD, RotationDHW}
	item := NewItem("custom", 10, 20, 30, 5, ItemAllowedRotations(rots))

	if len(item.AllowedRotations) != 2 {
		t.Fatalf("AllowedRotations count = %d, want 2", len(item.AllowedRotations))
	}
	if item.AllowedRotations[0] != RotationWHD || item.AllowedRotations[1] != RotationDHW {
		t.Errorf("AllowedRotations = %v, want [WHD, DHW]", item.AllowedRotations)
	}

	// Verify it's a copy (modifying original doesn't affect item).
	rots[0] = RotationHDW
	if item.AllowedRotations[0] != RotationWHD {
		t.Error("ItemAllowedRotations should copy the slice, not reference it")
	}
}

func TestRemoveLastItem(t *testing.T) {
	bin := NewBin("box", 20, 20, 20, 100)

	a := NewItem("a", 10, 10, 10, 5)
	b := NewItem("b", 10, 10, 10, 8, ItemFragile())

	bin.PlaceItem(a)
	bin.PlaceItem(b)

	if len(bin.Items) != 2 {
		t.Fatalf("Items count = %d, want 2", len(bin.Items))
	}
	if bin.TotalWeight() != 13 {
		t.Errorf("TotalWeight = %f, want 13", bin.TotalWeight())
	}
	if !bin.HasFragile {
		t.Error("expected HasFragile = true")
	}

	removed := bin.RemoveLastItem()
	if removed.ID != "b" {
		t.Errorf("removed ID = %q, want %q", removed.ID, "b")
	}
	if removed.Placed {
		t.Error("removed item should have Placed = false")
	}
	if len(bin.Items) != 1 {
		t.Errorf("Items count = %d, want 1", len(bin.Items))
	}
	if bin.TotalWeight() != 5 {
		t.Errorf("TotalWeight = %f, want 5", bin.TotalWeight())
	}
	if bin.HasFragile {
		t.Error("HasFragile should be false after removing only fragile item")
	}
	if len(bin.AABBData) != 6 {
		t.Errorf("AABBData len = %d, want 6", len(bin.AABBData))
	}

	// Remove last remaining item.
	removed = bin.RemoveLastItem()
	if removed.ID != "a" {
		t.Errorf("removed ID = %q, want %q", removed.ID, "a")
	}
	if len(bin.Items) != 0 {
		t.Errorf("Items count = %d, want 0", len(bin.Items))
	}
	if bin.TotalWeight() != 0 {
		t.Errorf("TotalWeight = %f, want 0", bin.TotalWeight())
	}
	if bin.UsedVolume() != 0 {
		t.Errorf("UsedVolume = %f, want 0", bin.UsedVolume())
	}
}

func TestVolumeUsedPctZeroVolumeBin(t *testing.T) {
	bin := NewBin("flat", 0, 10, 10, 100)
	if pct := bin.VolumeUsedPct(); pct != 0 {
		t.Errorf("VolumeUsedPct = %f, want 0 for zero-volume bin", pct)
	}
}

func TestPlaceItemFragileTracking(t *testing.T) {
	bin := NewBin("box", 30, 30, 30, 100)

	normal := NewItem("n", 10, 10, 10, 1)
	f1 := NewItem("f1", 10, 10, 10, 1, ItemFragile())
	f2 := NewItem("f2", 10, 10, 10, 1, ItemFragile())

	bin.PlaceItem(normal)
	if bin.HasFragile {
		t.Error("HasFragile should be false with no fragile items")
	}

	bin.PlaceItem(f1)
	bin.PlaceItem(f2)
	if len(bin.FragileIdxs) != 2 {
		t.Errorf("FragileIdxs = %v, want 2 entries", bin.FragileIdxs)
	}

	bin.RemoveLastItem() // remove f2
	if len(bin.FragileIdxs) != 1 {
		t.Errorf("FragileIdxs = %v, want 1 entry after removing f2", bin.FragileIdxs)
	}

	bin.RemoveLastItem() // remove f1
	if bin.HasFragile {
		t.Error("HasFragile should be false after removing all fragile items")
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
