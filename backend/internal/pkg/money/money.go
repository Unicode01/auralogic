package money

import (
	"fmt"
)

const (
	// CurrencyScale is the number of minor units in one major unit (e.g. cents).
	CurrencyScale int64 = 100
	// PercentageScale stores percentage as basis points.
	// 100% = 10000, 1% = 100.
	PercentageScale int64 = 10000
)

func ApplyPercentage(amountMinor, basisPoints int64) int64 {
	return amountMinor * basisPoints / PercentageScale
}

func MinorToString(amountMinor int64) string {
	sign := ""
	if amountMinor < 0 {
		sign = "-"
		amountMinor = -amountMinor
	}
	major := amountMinor / CurrencyScale
	minor := amountMinor % CurrencyScale
	return fmt.Sprintf("%s%d.%02d", sign, major, minor)
}

func FormatWithSymbol(amountMinor int64, currency string, symbols map[string]string) string {
	symbol := symbols[currency]
	if symbol == "" {
		symbol = currency + " "
	}
	return symbol + MinorToString(amountMinor)
}
