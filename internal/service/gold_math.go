package service

import "math"

// RoundTokenDelta floors to decimals (策划：floor，保留 2 位小数).
func RoundTokenDelta(raw float64, roundMode string, decimals int) float64 {
	if decimals <= 0 {
		decimals = 2
	}
	scale := math.Pow(10, float64(decimals))
	switch roundMode {
	case "round", "Round":
		return math.Round(raw*scale) / scale
	default: // floor
		return math.Floor(raw*scale) / scale
	}
}
