// Example: variable-sized bin packing (VSBPP)
//
// Demonstrates packing items into the best combination of available box types,
// minimizing total boxes used. Compares the fast TrialPacking approach with the
// Metaheuristic optimizer that redistributes items across bins.
//
// This is the most common real-world scenario: you have a catalog of box sizes
// and need to figure out which boxes to use for a given order.
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jcoruiz/gopackx"
	"github.com/jcoruiz/gopackx/pkg/model"
)

func main() {
	fmt.Println("=== Variable-Sized Bin Packing ===")
	fmt.Println()

	// --- 1. Quick pack (TrialPacking with lookahead) ---
	demoQuickPack()

	// --- 2. Optimized pack (Metaheuristic) ---
	demoOptimized()
}

// boxTypes returns the available box catalog.
func boxTypes() []*model.Bin {
	return []*model.Bin{
		model.NewBin("Bolsa Ecommerce", 43, 34.5, 5, 1),
		model.NewBin("Caja Pequeña", 25, 20, 20, 5),
		model.NewBin("Caja Mediana", 30, 30, 25, 10),
		model.NewBin("Caja Grande", 40, 40, 30, 20),
	}
}

// orderItems returns the items to pack (a sample office supply order).
func orderItems() []*model.Item {
	return []*model.Item{
		model.NewItem("Carpeta Argolla 1", 27, 8.5, 35.2, 0.395),
		model.NewItem("Carpeta Argolla 2", 27, 8.5, 35.2, 0.395),
		model.NewItem("Carpeta Argolla 3", 27, 8.5, 35.2, 0.395),
		model.NewItem("Carpeta Pres 1", 27, 9, 30.5, 0.415),
		model.NewItem("Carpeta Pres 2", 27, 9, 30.5, 0.415),
		model.NewItem("Carpeta Pres 3", 27, 9, 30.5, 0.415),
		model.NewItem("Carpeta Pres 4", 27, 9, 30.5, 0.415),
		model.NewItem("Carpeta Pres 5", 27, 9, 30.5, 0.415),
		model.NewItem("Cuaderno Prof 1", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("Cuaderno Prof 2", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("Cuaderno Prof 3", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("Cuaderno Prof 4", 28.5, 6.8, 29.4, 0.534),
		model.NewItem("Resma Carta 1", 23.5, 6, 34.5, 2.780),
		model.NewItem("Resma Carta 2", 23.5, 6, 34.5, 2.780),
		model.NewItem("Resma Carta 3", 23.5, 6, 34.5, 2.780),
		model.NewItem("Papel Bond 1", 28, 5, 22, 2.500),
		model.NewItem("Papel Bond 2", 28, 5, 22, 2.500),
	}
}

func printResult(result *model.Result) {
	fmt.Printf("  Items:    %d / %d fitted\n", result.Stats.FittedItems, result.Stats.TotalItems)
	fmt.Printf("  Boxes:    %d\n", result.Stats.TotalBins)
	fmt.Printf("  Avg fill: %.1f%%\n", result.Stats.VolumeUsedPct)
	fmt.Println()

	for _, bin := range result.Bins {
		if len(bin.Items) == 0 {
			continue
		}
		fmt.Printf("  %s (%gx%gx%g)\n", bin.ID, bin.Width, bin.Height, bin.Depth)
		fmt.Printf("    %d items | %.1f%% vol | %.2f / %.0f kg\n",
			len(bin.Items), bin.VolumeUsedPct(), bin.TotalWeight(), bin.MaxWeight)
		for _, item := range bin.Items {
			dim := item.Dimension()
			fmt.Printf("      %-20s  %.1f×%.1f×%.1f\n", item.ID, dim[0], dim[1], dim[2])
		}
		fmt.Println()
	}

	if len(result.UnfittedItems) > 0 {
		fmt.Printf("  Unfitted: %d items\n", len(result.UnfittedItems))
	}
}

func demoQuickPack() {
	fmt.Println("--- 1. Quick Pack (TrialPacking + Lookahead) ---")
	fmt.Println("Fast bin-type selection using simulated trial packing.")
	fmt.Println()

	start := time.Now()
	result, err := gopackx.Pack(context.Background(), boxTypes(), orderItems())
	elapsed := time.Since(start)

	if err != nil {
		log.Fatal(err)
	}

	printResult(result)
	fmt.Printf("  Time: %s\n\n", elapsed.Round(time.Microsecond))
}

func demoOptimized() {
	fmt.Println("--- 2. Optimized Pack (Metaheuristic) ---")
	fmt.Println("VNS-based optimizer that redistributes items across bins.")
	fmt.Println()

	start := time.Now()
	result, err := gopackx.Pack(context.Background(), boxTypes(), orderItems(),
		gopackx.Optimize(),
	)
	elapsed := time.Since(start)

	if err != nil {
		log.Fatal(err)
	}

	printResult(result)
	fmt.Printf("  Time: %s\n\n", elapsed.Round(time.Microsecond))
}
