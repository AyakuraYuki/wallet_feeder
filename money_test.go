package main

import (
	"errors"
	"testing"
)

func TestParseCents(t *testing.T) {
	tests := []struct {
		in   string
		want int64
		ok   bool
	}{
		{"36", 3600, true},
		{"36.5", 3650, true},
		{"36.50", 3650, true},
		{"0.01", 1, true},
		{"¥1,234.56", 123456, true},
		{" 12 ", 1200, true},
		{"36.", 3600, true},
		{"-1", 0, false},
		{"−1", 0, false},
		{"0", 0, true},
		{"", 0, false},
		{"abc", 0, false},
		{"1.234", 0, false},
		{"1,23", 12300, true}, // grouping commas are stripped
		{".5", 0, false},
		{"+12", 1200, true},
		{"1..2", 0, false},
	}
	for _, tt := range tests {
		got, err := ParseCents(tt.in, false)
		if tt.ok {
			if err != nil || got != tt.want {
				t.Errorf("ParseCents(%q) = %d, %v; want %d, nil", tt.in, got, err, tt.want)
			}
		} else if !errors.Is(err, ErrAmountFormat) && !errors.Is(err, ErrAmountTooPrecise) &&
			!errors.Is(err, ErrAmountNegative) && !errors.Is(err, ErrEmptyAmount) {
			t.Errorf("ParseCents(%q) error = %v, want a parse error", tt.in, err)
		}
	}
}

func TestParseSignedCents(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want int64
	}{
		{"-12.30", -1230},
		{"−12.30", -1230},
		{"0", 0},
		{"¥18", 1800},
	} {
		got, err := ParseBalanceCents(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("ParseBalanceCents(%q) = %d, %v; want %d, nil", tt.in, got, err, tt.want)
		}
	}
}

func TestParsePositiveCentsRejectsZero(t *testing.T) {
	if _, err := ParsePositiveCents("0"); !errors.Is(err, ErrAmountNotPositive) {
		t.Fatalf("zero amount error = %v, want ErrAmountNotPositive", err)
	}
	if _, err := ParsePositiveCents("0.01"); err != nil {
		t.Fatalf("one cent: %v", err)
	}
}

func TestFormatCents(t *testing.T) {
	for _, tt := range []struct {
		cents int64
		want  string
	}{
		{0, "¥0.00"},
		{1, "¥0.01"},
		{123456, "¥1,234.56"},
		{-123456, "−¥1,234.56"},
		{100000000, "¥1,000,000.00"},
	} {
		if got := FormatCents(tt.cents); got != tt.want {
			t.Errorf("FormatCents(%d) = %q, want %q", tt.cents, got, tt.want)
		}
	}
	if got := FormatDelta(500); got != "+¥5.00" {
		t.Errorf("FormatDelta(500) = %q", got)
	}
	if got := FormatDelta(-500); got != "−¥5.00" {
		t.Errorf("FormatDelta(-500) = %q", got)
	}
}
