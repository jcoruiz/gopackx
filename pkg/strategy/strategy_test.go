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

func TestSortBinsWeightFilter(t *testing.T) {
	bin := model.NewBin("b1", 10, 10, 10, 5)
	item := model.NewItem("heavy", 5, 5, 5, 10)

	result := SortBinsForItem([]*model.Bin{bin}, item, MinimizeBins)
	if len(result) != 0 {
		t.Error("should exclude bins with insufficient weight capacity")
	}
}
