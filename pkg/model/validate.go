package model

import (
	"errors"
	"math"
	"strconv"
)

// ErrInvalidInput is wrapped by every error Validate returns, so callers can
// tell bad input apart from other failures with errors.Is.
var ErrInvalidInput = errors.New("invalid input")

// ValidationError describes the first invalid value found in a bin or item.
type ValidationError struct {
	Kind   string // "bin" or "item"
	Index  int    // position in the slice that was validated
	ID     string
	Field  string // e.g. "Width"; empty when the bin or item itself is nil
	Reason string
}

func (e *ValidationError) Error() string {
	msg := "gopackx: " + e.Kind + " " + strconv.Quote(e.ID) + " (index " + strconv.Itoa(e.Index) + "): "
	if e.Field != "" {
		msg += e.Field + " "
	}
	return msg + e.Reason
}

// Unwrap makes errors.Is(err, ErrInvalidInput) report true.
func (e *ValidationError) Unwrap() error { return ErrInvalidInput }

// Validate checks bins and items before packing. Dimensions must be finite
// and greater than zero; weights, weight limits, costs and load limits must
// be finite and not negative; an item needs at least one allowed rotation
// and only valid ones. Without these checks a NaN dimension or a negative
// weight would be packed silently, for example letting a box carry more
// than its weight limit.
//
// It returns nil or the first problem found, as a *ValidationError.
func Validate(bins []*Bin, items []*Item) error {
	for i, b := range bins {
		if err := validateBin(i, b); err != nil {
			return err
		}
	}
	for i, it := range items {
		if err := validateItem(i, it); err != nil {
			return err
		}
	}
	return nil
}

func validateBin(i int, b *Bin) error {
	if b == nil {
		return &ValidationError{Kind: "bin", Index: i, Reason: "is nil"}
	}
	fail := func(field, reason string) error {
		return &ValidationError{Kind: "bin", Index: i, ID: b.ID, Field: field, Reason: reason}
	}
	for _, d := range []struct {
		name string
		v    float64
	}{{"Width", b.Width}, {"Height", b.Height}, {"Depth", b.Depth}} {
		if !positive(d.v) {
			return fail(d.name, mustBePositive(d.v))
		}
	}
	if !nonNegative(b.MaxWeight) {
		return fail("MaxWeight", mustBeNonNegative(b.MaxWeight))
	}
	if !nonNegative(b.Cost) {
		return fail("Cost", mustBeNonNegative(b.Cost))
	}
	return nil
}

func validateItem(i int, it *Item) error {
	if it == nil {
		return &ValidationError{Kind: "item", Index: i, Reason: "is nil"}
	}
	fail := func(field, reason string) error {
		return &ValidationError{Kind: "item", Index: i, ID: it.ID, Field: field, Reason: reason}
	}
	for _, d := range []struct {
		name string
		v    float64
	}{{"Width", it.Width}, {"Height", it.Height}, {"Depth", it.Depth}} {
		if !positive(d.v) {
			return fail(d.name, mustBePositive(d.v))
		}
	}
	if !nonNegative(it.Weight) {
		return fail("Weight", mustBeNonNegative(it.Weight))
	}
	if !nonNegative(it.LoadBear) {
		return fail("LoadBear", mustBeNonNegative(it.LoadBear))
	}
	if len(it.AllowedRotations) == 0 {
		return fail("AllowedRotations", "is empty: the item could never be placed")
	}
	for _, rt := range it.AllowedRotations {
		if rt < RotationWHD || rt > RotationWDH {
			return fail("AllowedRotations", "contains unknown rotation "+strconv.Itoa(int(rt)))
		}
	}
	return nil
}

func positive(v float64) bool    { return v > 0 && !math.IsInf(v, 0) }
func nonNegative(v float64) bool { return v >= 0 && !math.IsInf(v, 0) }

func mustBePositive(v float64) string {
	return "must be a finite number greater than 0, got " + strconv.FormatFloat(v, 'g', -1, 64)
}

func mustBeNonNegative(v float64) string {
	return "must be a finite number not below 0, got " + strconv.FormatFloat(v, 'g', -1, 64)
}
