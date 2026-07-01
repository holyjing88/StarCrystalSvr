package service

import (
	"testing"
	"time"
)

func TestIsPenultimateExchangeDay(t *testing.T) {
	loc := GoldLocation("")
	// Jan 2026: last day 31, penultimate 30
	d30 := time.Date(2026, 1, 30, 12, 0, 0, 0, loc)
	if !IsPenultimateExchangeDay(d30) {
		t.Fatal("Jan 30 should be penultimate")
	}
	d31 := time.Date(2026, 1, 31, 12, 0, 0, 0, loc)
	if IsPenultimateExchangeDay(d31) {
		t.Fatal("Jan 31 should not be penultimate")
	}
	// Feb 2026 (28 days): penultimate 27
	d27 := time.Date(2026, 2, 27, 12, 0, 0, 0, loc)
	if !IsPenultimateExchangeDay(d27) {
		t.Fatal("Feb 27 2026 should be penultimate")
	}
}
