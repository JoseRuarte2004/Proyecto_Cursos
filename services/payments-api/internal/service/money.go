package service

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

var ErrInvalidAmount = errors.New("invalid amount")

func amountToCents(amount float64) (int64, error) {
	if math.IsNaN(amount) || math.IsInf(amount, 0) {
		return 0, ErrInvalidAmount
	}

	scaled := math.Round(amount * 100)
	if scaled < 0 {
		return 0, ErrInvalidAmount
	}

	return int64(scaled), nil
}

func centsToAmount(amountCents int64) float64 {
	return float64(amountCents) / 100
}

func numericTextToCents(value string) (int64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0, ErrInvalidAmount
	}

	negative := false
	if strings.HasPrefix(trimmed, "-") {
		negative = true
		trimmed = strings.TrimPrefix(trimmed, "-")
	}

	parts := strings.SplitN(trimmed, ".", 3)
	if len(parts) > 2 {
		return 0, fmt.Errorf("%w: %q", ErrInvalidAmount, value)
	}

	wholePart := parts[0]
	if wholePart == "" {
		wholePart = "0"
	}

	whole, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidAmount, err)
	}

	fractionPart := "00"
	if len(parts) == 2 {
		switch len(parts[1]) {
		case 0:
			fractionPart = "00"
		case 1:
			fractionPart = parts[1] + "0"
		default:
			fractionPart = parts[1][:2]
		}
	}

	fraction, err := strconv.ParseInt(fractionPart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %v", ErrInvalidAmount, err)
	}

	total := whole*100 + fraction
	if negative {
		total *= -1
	}

	return total, nil
}

func centsToNumericText(amountCents int64) string {
	sign := ""
	value := amountCents
	if value < 0 {
		sign = "-"
		value *= -1
	}

	return fmt.Sprintf("%s%d.%02d", sign, value/100, value%100)
}

func CentsToAmount(amountCents int64) float64 {
	return centsToAmount(amountCents)
}

func CentsToNumericText(amountCents int64) string {
	return centsToNumericText(amountCents)
}

func NumericTextToCents(value string) (int64, error) {
	return numericTextToCents(value)
}
