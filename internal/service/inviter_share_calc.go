package service

import (
	"math"
	"strings"

	"starcrystal/server/internal/config"
)

const platformBeneficiaryID = "platform"

type shareLayerAmounts struct {
	Raw, Paid, Platform float64
}

type computedShareResult struct {
	Direct, Second                           shareLayerAmounts
	TotalPaid, TotalPlatform, DenominatorAdd float64
}

func calcInviteShare(base float64, invite config.InviteConfig, gmMode string, isGM bool) computedShareResult {
	if base <= 0 {
		return computedShareResult{}
	}
	decimals := invite.ShareDecimals
	if decimals <= 0 {
		decimals = 2
	}
	rawDirect := roundShare(base*invite.DirectShareRate, decimals, invite.ShareRound)
	rawSecond := roundShare(base*invite.SecondShareRate, decimals, invite.ShareRound)
	mode := strings.ToLower(strings.TrimSpace(gmMode))
	if isGM && mode == "none" {
		p := rawDirect + rawSecond
		return computedShareResult{
			Direct:        shareLayerAmounts{Raw: rawDirect, Platform: rawDirect},
			Second:        shareLayerAmounts{Raw: rawSecond, Platform: rawSecond},
			TotalPlatform: p, DenominatorAdd: p,
		}
	}
	maxDirect, maxSecond, maxTotal := invite.MaxDirectSharePerEvent, invite.MaxSecondSharePerEvent, invite.MaxTotalSharePerEvent
	if isGM && mode == "capped" {
		maxDirect = invite.GmMaxDirectSharePerEvent
		maxSecond = invite.GmMaxSecondSharePerEvent
		maxTotal = invite.GmMaxTotalSharePerEvent
	}
	payDirect := applyLayerCap(rawDirect, maxDirect)
	paySecond := applyLayerCap(rawSecond, maxSecond)
	plat := (rawDirect - payDirect) + (rawSecond - paySecond)
	if maxTotal > 0 {
		sum := payDirect + paySecond
		if sum > maxTotal && sum > 0 {
			ratio := maxTotal / sum
			newDirect := roundShare(payDirect*ratio, decimals, "floor")
			newSecond := roundShare(paySecond*ratio, decimals, "floor")
			plat += (payDirect + paySecond) - (newDirect + newSecond)
			payDirect, paySecond = newDirect, newSecond
		}
	}
	totalPaid := payDirect + paySecond
	return computedShareResult{
		Direct:    shareLayerAmounts{Raw: rawDirect, Paid: payDirect, Platform: rawDirect - payDirect},
		Second:    shareLayerAmounts{Raw: rawSecond, Paid: paySecond, Platform: rawSecond - paySecond},
		TotalPaid: totalPaid, TotalPlatform: plat, DenominatorAdd: totalPaid + plat,
	}
}

func applyLayerCap(raw, cap float64) float64 {
	if cap <= 0 || raw <= cap {
		return raw
	}
	return cap
}

func roundShare(v float64, decimals int, mode string) float64 {
	if decimals < 0 {
		decimals = 0
	}
	pow := math.Pow(10, float64(decimals))
	scaled := v * pow
	if strings.ToLower(strings.TrimSpace(mode)) == "ceil" {
		return math.Ceil(scaled) / pow
	}
	return math.Floor(scaled) / pow
}
