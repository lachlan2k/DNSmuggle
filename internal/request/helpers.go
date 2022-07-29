package request

import "math"

// One byte for the master header, eight for sess id, two for frag header
const requestOverhead = 1 + 8 + 2
const periodOverheadRatio = 63.0 / 64.0 // we need to place a period ever 63 characters
const b32OverheadRatio = 5.0 / 8.0      // base32 loses 3 bits of space efficiency
const b64OverheadRatio = 6.0 / 8.0

func GetMaxRequestSize(domain string) int {
	return int(math.Floor(periodOverheadRatio*math.Floor(b32OverheadRatio*float64(254-len(domain))))) - requestOverhead
}

func GetMaxResponseSize() int {
	return 200 * b64OverheadRatio
}
