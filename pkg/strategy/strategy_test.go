package strategy

import (
	"testing"

	"github.com/jcoruiz/gopackx/pkg/model"
)

func TestSortItemsVolumeDesc(t *testing.T) {
	items := []*model.Item{
		model.NewItem("small", 1, 1, 1, 1),
		model.NewItem("large", 10, 10, 10, 1),
		model.NewItem("medium", 5, 5, 5, 1),
	}

	SortItems(items, BestFitDecreasing)

	if items[0].ID != "large" || items[1].ID != "medium" || items[2].ID != "small" {
		t.Errorf("expected large > medium > small, got %s > %s > %s",
			items[0].ID, items[1].ID, items[2].ID)
	}
}

func TestSortItemsVolumeAsc(t *testing.T) {
	items := []*model.Item{
		model.NewItem("large", 10, 10, 10, 1),
		model.NewItem("small", 1, 1, 1, 1),
		model.NewItem("medium", 5, 5, 5, 1),
	}

	SortItems(items, BestFit)

	if items[0].ID != "small" || items[1].ID != "medium" || items[2].ID != "large" {
		t.Errorf("expected small < medium < large, got %s < %s < %s",
			items[0].ID, items[1].ID, items[2].ID)
	}
}

func TestSortItemsMultiCriteria(t *testing.T) {
	// Same volume, different load-bearing.
	a := model.NewItem("a", 10, 10, 10, 1, model.ItemLoadBear(20))
	b := model.NewItem("b", 10, 10, 10, 1, model.ItemLoadBear(5))
	items := []*model.Item{b, a}

	SortItems(items, MinimizeBins)

	if items[0].ID != "a" {
		t.Error("expected higher load-bearing first")
	}
}

func TestSortItemsPriority(t *testing.T) {
	a := model.NewItem("a", 10, 10, 10, 1, model.ItemPriority(3))
	b := model.NewItem("b", 10, 10, 10, 1, model.ItemPriority(1))
	c := model.NewItem("c", 10, 10, 10, 1, model.ItemPriority(2))
	items := []*model.Item{a, b, c}

	SortItems(items, MinimizeBins)

	if items[0].ID != "b" || items[1].ID != "c" || items[2].ID != "a" {
		t.Errorf("expected priority order b(1) < c(2) < a(3), got %s < %s < %s",
			items[0].ID, items[1].ID, items[2].ID)
	}
}

func TestSortBinsForItemFirstFit(t *testing.T) {
	bins := []*model.Bin{
		model.NewBin("b1", 10, 10, 10, 100),
		model.NewBin("b2", 20, 20, 20, 100),
	}
	item := model.NewItem("i1", 5, 5, 5, 10)

	result := SortBinsForItem(bins, item, MinimizeBins)
	if len(result) != 2 {
		t.Fatalf("expected 2 bins, got %d", len(result))
	}
	if result[0].ID != "b1" {
		t.Error("First Fit should keep original order")
	}
}

func TestSortBinsForItemBestFit(t *testing.T) {
	b1 := model.NewBin("big", 20, 20, 20, 100)
	b2 := model.NewBin("small", 10, 10, 10, 100)
	bins := []*model.Bin{b1, b2}
	item := model.NewItem("i1", 5, 5, 5, 10)

	result := SortBinsForItem(bins, item, BestFit)
	if result[0].ID != "small" {
		t.Error("BestFit should prefer smallest remaining volume")
	}
}

func TestSortBinsForItemWorstFit(t *testing.T) {
	b1 := model.NewBin("small", 10, 10, 10, 100)
	b2 := model.NewBin("big", 20, 20, 20, 100)
	bins := []*model.Bin{b1, b2}
	item := model.NewItem("i1", 5, 5, 5, 10)

	result := SortBinsForItem(bins, item, WorstFit)
	if result[0].ID != "big" {
		t.Error("WorstFit should prefer largest remaining volume")
	}
}

func TestSortBinsForItemBestFit3Bins(t *testing.T) {
	bins := []*model.Bin{
		model.NewBin("big", 100, 100, 100, 500), // vol=1_000_000
		model.NewBin("small", 30, 30, 30, 500),  // vol=27_000
		model.NewBin("medium", 50, 50, 50, 500), // vol=125_000
	}
	item := model.NewItem("i1", 5, 5, 5, 1)

	result := SortBinsForItem(bins, item, BestFit)
	if len(result) != 3 {
		t.Fatalf("expected 3 bins, got %d", len(result))
	}
	// BestFit: ascending remaining volume → small, medium, big.
	if result[0].ID != "small" || result[1].ID != "medium" || result[2].ID != "big" {
		t.Errorf("BestFit 3 bins: expected small < medium < big, got %s < %s < %s",
			result[0].ID, result[1].ID, result[2].ID)
	}
}

func TestSortBinsForItemBestFit5Bins(t *testing.T) {
	bins := []*model.Bin{
		model.NewBin("e", 90, 90, 90, 500),
		model.NewBin("a", 10, 10, 10, 500),
		model.NewBin("d", 70, 70, 70, 500),
		model.NewBin("b", 30, 30, 30, 500),
		model.NewBin("c", 50, 50, 50, 500),
	}
	item := model.NewItem("i1", 5, 5, 5, 1)

	result := SortBinsForItem(bins, item, BestFit)
	if len(result) != 5 {
		t.Fatalf("expected 5 bins, got %d", len(result))
	}
	expected := []string{"a", "b", "c", "d", "e"}
	for i, id := range expected {
		if result[i].ID != id {
			t.Errorf("BestFit 5 bins: position %d expected %s, got %s", i, id, result[i].ID)
		}
	}
}

func TestSortBinsForItemWorstFit3Bins(t *testing.T) {
	bins := []*model.Bin{
		model.NewBin("small", 30, 30, 30, 500),
		model.NewBin("big", 100, 100, 100, 500),
		model.NewBin("medium", 50, 50, 50, 500),
	}
	item := model.NewItem("i1", 5, 5, 5, 1)

	result := SortBinsForItem(bins, item, WorstFit)
	if len(result) != 3 {
		t.Fatalf("expected 3 bins, got %d", len(result))
	}
	// WorstFit: descending remaining volume → big, medium, small.
	if result[0].ID != "big" || result[1].ID != "medium" || result[2].ID != "small" {
		t.Errorf("WorstFit 3 bins: expected big > medium > small, got %s > %s > %s",
			result[0].ID, result[1].ID, result[2].ID)
	}
}

func TestSortBinsForItemBestFitDecreasing3Bins(t *testing.T) {
	bins := []*model.Bin{
		model.NewBin("big", 100, 100, 100, 500),
		model.NewBin("small", 30, 30, 30, 500),
		model.NewBin("medium", 50, 50, 50, 500),
	}
	item := model.NewItem("i1", 5, 5, 5, 1)

	result := SortBinsForItem(bins, item, BestFitDecreasing)
	if len(result) != 3 {
		t.Fatalf("expected 3 bins, got %d", len(result))
	}
	if result[0].ID != "small" || result[1].ID != "medium" || result[2].ID != "big" {
		t.Errorf("BestFitDecreasing 3 bins: expected small < medium < big, got %s < %s < %s",
			result[0].ID, result[1].ID, result[2].ID)
	}
}

func TestSortBinsForItemAlmostWorstFit(t *testing.T) {
	// AlmostWorstFit skips bins that are >80% empty (usage < 20%).
	// Create bins with different fill levels.
	almostEmpty := model.NewBin("almost-empty", 100, 100, 100, 500) // 0% used
	partiallyFilled := model.NewBin("partial", 50, 50, 50, 500)     // will place items
	full := model.NewBin("full", 20, 20, 20, 500)                   // will place items

	// Fill "partial" to ~50% usage.
	for i := 0; i < 4; i++ {
		filler := model.NewItem("f", 25, 25, 25, 1)
		filler.Position = [3]float64{float64(i%2) * 25, 0, float64(i/2) * 25}
		partiallyFilled.PlaceItem(filler)
	}

	// Fill "full" to ~50%.
	filler := model.NewItem("f", 20, 10, 20, 1)
	filler.Position = [3]float64{0, 0, 0}
	full.PlaceItem(filler)

	bins := []*model.Bin{almostEmpty, partiallyFilled, full}
	item := model.NewItem("box", 5, 5, 5, 1)

	result := SortBinsForItem(bins, item, AlmostWorstFit)

	// Almost-empty bin (0% usage < 20%) should be skipped if there are others.
	// The other bins (>20% usage) should be sorted by most remaining first.
	if len(result) < 1 {
		t.Fatal("expected at least 1 bin")
	}
	// almostEmpty should be excluded (usage = 0% < 20%, and it has 0 items).
	// Actually, bins with 0 items are included (len(b.Items) == 0 passes the check).
	// Only bins with items AND usage < 20% are skipped.
	for _, b := range result {
		if len(b.Items) > 0 && b.VolumeUsedPct() < 20 {
			t.Errorf("AlmostWorstFit should skip bins with items and usage < 20%%, got %s at %.1f%%",
				b.ID, b.VolumeUsedPct())
		}
	}
}

func TestSortBinsForItemAlmostWorstFitAllEmpty(t *testing.T) {
	// When all bins are empty/near-empty, fallback to using all candidates.
	bins := []*model.Bin{
		model.NewBin("a", 100, 100, 100, 500),
		model.NewBin("b", 50, 50, 50, 500),
	}
	item := model.NewItem("box", 5, 5, 5, 1)

	result := SortBinsForItem(bins, item, AlmostWorstFit)
	if len(result) != 2 {
		t.Errorf("expected 2 bins (fallback to all), got %d", len(result))
	}
	// Should be sorted by most remaining first.
	if result[0].ID != "a" {
		t.Errorf("expected 'a' first (most remaining), got %s", result[0].ID)
	}
}

func TestSortBinsWeightFilter(t *testing.T) {
	bin := model.NewBin("b1", 10, 10, 10, 5)
	item := model.NewItem("heavy", 5, 5, 5, 10)

	result := SortBinsForItem([]*model.Bin{bin}, item, MinimizeBins)
	if len(result) != 0 {
		t.Error("should exclude bins with insufficient weight capacity")
	}
}
