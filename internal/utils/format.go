package utils

import (
	"fmt"
	"math"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// FormatPriceCompact formats large numbers with K/M/B suffixes for compact display.
// Examples:
//   - 1,500,000,000 → "$1.5B"
//   - 5,500,000 → "$5.5M"
//   - 34,500 → "$34.5K"
//   - 123.45 → "$123.45"
//
// Negative values are prefixed with a minus sign.
func FormatPriceCompact(value float64) string {
	p := message.NewPrinter(language.AmericanEnglish)

	sign := ""
	if value < 0 {
		sign = "-"
		value = math.Abs(value)
	}

	switch {
	case value >= 1_000_000_000:
		v := math.Round(value/1_000_000_000*10) / 10
		return sign + p.Sprintf("$%.1fB", v)

	case value >= 1_000_000:
		v := math.Round(value/1_000_000*10) / 10
		return sign + p.Sprintf("$%.1fM", v)

	case value >= 1_000:
		v := math.Round(value/1_000*10) / 10
		return sign + p.Sprintf("$%.1fK", v)

	default:
		v := math.Round(value*100) / 100
		return sign + p.Sprintf("$%.2f", v)
	}
}

// FormatPrice formats price with $ prefix and adaptive decimal precision.
// The number of decimal places adapts based on price magnitude:
//   - Very small (< 0.01): shows significant digits (e.g., "$0.0003456")
//   - Small (0.01 - 1): 2-4 decimals without trailing zeros (e.g., "$0.15")
//   - Normal (>= 1): 2 decimals without trailing zeros (e.g., "$3" or "$3.50")
//
// Thousand separators are automatically added using American English formatting.
// Examples:
//   - 0.0003456 → "$0.0003456"
//   - 0.7248 → "$0.7248"
//   - 3.00 → "$3"
//   - 90735.00 → "$90,735"
func FormatPrice(value float64) string {
	p := message.NewPrinter(language.AmericanEnglish)

	absValue := math.Abs(value)
	sign := ""
	if value < 0 {
		sign = "-"
	}

	var formatted string

	switch {
	case absValue == 0:
		formatted = p.Sprintf("$0")

	case absValue < 0.01:
		// For very small numbers, find first significant digit
		// and show enough decimals to display 4-5 significant figures
		decimals := 2
		temp := absValue
		for temp < 1 && decimals < 10 {
			temp *= 10
			decimals++
		}
		// Add 3 more decimals to show significant figures
		decimals += 3

		// Format with calculated decimals, then trim trailing zeros
		formatStr := fmt.Sprintf("%%.%df", decimals)
		formatted = fmt.Sprintf(formatStr, absValue)
		formatted = trimTrailingZeros(formatted)
		formatted = p.Sprintf("$%s", formatted)

	case absValue < 1:
		// For prices between 0.01 and 1, show 2-4 decimals without trailing zeros
		formatted = fmt.Sprintf("%.4f", absValue)
		formatted = trimTrailingZeros(formatted)
		formatted = p.Sprintf("$%s", formatted)

	default:
		// For normal and large prices, show 2 decimals with thousand separators
		formatted = p.Sprintf("%.2f", absValue)
		formatted = trimTrailingZeros(formatted)
		formatted = p.Sprintf("$%s", formatted)
	}

	return sign + formatted
}

// FormatFundingRate formats a funding rate as a percentage with 5 decimal places.
// The input rate is expected to be in decimal form (e.g., 0.0000125).
// Example: 0.0000125 → "0.00125%"
func FormatFundingRate(rate float64) string {
	// Convert to percentage (rate is already decimal, e.g., 0.0000125)
	percentage := rate * 100

	// Format with 5 decimal places
	return fmt.Sprintf("%.5f%%", percentage)
}

// Format24HChange formats the 24-hour price change with both absolute and percentage values.
// Returns a formatted string with sign prefix.
// Examples:
//   - diffPrice=8.4, diffPercent=0.27 → "+8.4 / +0.27%"
//   - diffPrice=-0.0104, diffPercent=-0.20 → "-0.0104 / -0.20%"
func Format24HChange(diffPrice float64, diffPercent float64) string {
	p := message.NewPrinter(language.AmericanEnglish)

	sign := ""
	if diffPrice >= 0 {
		sign = "+"
	}

	// Format price difference
	var priceStr string
	if math.Abs(diffPrice) >= 1 {
		priceStr = p.Sprintf("%.1f", diffPrice)
	} else {
		priceStr = fmt.Sprintf("%.5f", diffPrice)
	}

	// Format percentage
	percentStr := fmt.Sprintf("%.2f%%", diffPercent)
	if diffPercent >= 0 {
		percentStr = "+" + percentStr
	}

	return sign + priceStr + " / " + percentStr
}

// trimTrailingZeros removes trailing zeros from a decimal number string.
// If all decimal digits are removed, the decimal point is also removed.
// Examples:
//   - "3.0000" → "3"
//   - "3.5000" → "3.5"
//   - "0.1230" → "0.123"
func trimTrailingZeros(s string) string {
	if !containsDecimal(s) {
		return s
	}

	// Trim trailing zeros
	s = strings.TrimRight(s, "0")

	// If we removed all decimals, remove the decimal point too
	s = strings.TrimRight(s, ".")

	return s
}

// containsDecimal checks if a string contains a decimal point.
func containsDecimal(s string) bool {
	return strings.Contains(s, ".")
}
