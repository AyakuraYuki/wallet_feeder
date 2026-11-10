package main

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Money is kept as a signed count of cents everywhere: never as a float.
const (
	// MaxAmountCents bounds a single amount and a monthly budget:
	// ¥9,999,999,999.99. It keeps every sum far inside int64.
	MaxAmountCents int64 = 999_999_999_999
	centsPerYuan   int64 = 100
)

var (
	ErrEmptyAmount       = errors.New("amount is empty")
	ErrAmountFormat      = errors.New("amount is not a number")
	ErrAmountTooPrecise  = errors.New("amount has more than two decimal places")
	ErrAmountTooLarge    = errors.New("amount is too large")
	ErrAmountNotPositive = errors.New("amount must be greater than zero")
	ErrAmountNegative    = errors.New("amount may not be negative")
)

// ParseCents reads a yuan amount such as "36", "36.5", "36.50" or
// "¥1,234.50" into cents. Negative values are read only when allowNegative
// is true, as for a wallet calibration. Empty text is an error.
func ParseCents(s string, allowNegative bool) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "¥")
	s = strings.TrimPrefix(s, "￥")
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, ErrEmptyAmount
	}

	negative := false
	switch {
	case strings.HasPrefix(s, "-"):
		negative = true
		s = s[1:]
	case strings.HasPrefix(s, "−"):
		negative = true
		s = strings.TrimPrefix(s, "−")
	case strings.HasPrefix(s, "+"):
		s = s[1:]
	}
	if s == "" {
		return 0, ErrAmountFormat
	}
	if negative && !allowNegative {
		return 0, ErrAmountNegative
	}
	if strings.HasPrefix(s, "-") || strings.HasPrefix(s, "−") || strings.HasPrefix(s, "+") {
		return 0, ErrAmountFormat
	}

	// Accept groups of three digits separated by commas ("1,234.50") but
	// only when they are actually grouped that way.
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "，", "")

	whole, frac, hasDot := strings.Cut(s, ".")
	if whole == "" {
		return 0, ErrAmountFormat
	}
	if hasDot && len(frac) > 2 {
		return 0, ErrAmountTooPrecise
	}
	for _, r := range whole {
		if r < '0' || r > '9' {
			return 0, ErrAmountFormat
		}
	}
	for _, r := range frac {
		if r < '0' || r > '9' {
			return 0, ErrAmountFormat
		}
	}
	wholeN, err := strconv.ParseInt(whole, 10, 64)
	if err != nil || wholeN > MaxAmountCents/centsPerYuan {
		return 0, ErrAmountTooLarge
	}
	var fracN int64
	for _, r := range frac {
		fracN = fracN*10 + int64(r-'0')
	}
	for i := len(frac); i < 2; i++ {
		fracN *= 10
	}
	cents := wholeN*centsPerYuan + fracN
	if cents > MaxAmountCents {
		return 0, ErrAmountTooLarge
	}
	if negative {
		cents = -cents
	}
	return cents, nil
}

// ParsePositiveCents reads a strictly positive amount for a transaction or
// a budget.
func ParsePositiveCents(s string) (int64, error) {
	cents, err := ParseCents(s, false)
	if err != nil {
		return 0, err
	}
	if cents <= 0 {
		return 0, ErrAmountNotPositive
	}
	return cents, nil
}

// ParseBalanceCents reads a wallet balance, which may be negative.
func ParseBalanceCents(s string) (int64, error) {
	return ParseCents(s, true)
}

// FormatCents writes an amount as "¥1,234.50", and a negative one as
// "−¥12.00". The grouping uses commas; tests and the ledger use the same
// text.
func FormatCents(cents int64) string {
	sign := ""
	if cents < 0 {
		sign = "−"
		cents = -cents
	}
	whole := cents / centsPerYuan
	frac := cents % centsPerYuan
	return sign + "¥" + groupDigits(whole) + "." + fmt.Sprintf("%02d", frac)
}

// FormatDelta writes a difference with an explicit sign: "+¥12.00",
// "−¥3.50", or "¥0.00".
func FormatDelta(cents int64) string {
	switch {
	case cents > 0:
		return "+" + FormatCents(cents)
	case cents < 0:
		return FormatCents(cents)
	default:
		return FormatCents(0)
	}
}

// FormatCentsCompact writes "¥1,234" without cents for a wallet that has
// none, and the usual format otherwise. The main overview always uses the
// exact format; this is for tight chips.
func FormatCentsCompact(cents int64) string {
	if cents%centsPerYuan == 0 {
		sign := ""
		if cents < 0 {
			sign = "−"
			cents = -cents
		}
		return sign + "¥" + groupDigits(cents/centsPerYuan)
	}
	return FormatCents(cents)
}

// groupDigits inserts a comma between every group of three digits.
func groupDigits(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// clampPercent rounds a percentage and keeps it in 0..100 for layout.
func clampPercent(v float64) float64 {
	if math.IsNaN(v) || v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
