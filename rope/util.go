package rope

import (
	"math/bits"
	"math/rand/v2"
)

// randomHeight picks a height in the range [1,32], inclusive.
// The odds of returning 1 is 50%, 2 is 25%, 3 is 12.5%, and so on.
func randomHeight() int {
	// 1 + TrailingZeros is a geometric distribution.
	// We cap it at maxHeight (32).
	// rand.Uint32() can be zero, in which case TrailingZeros32 is 32.
	// So h can be at most 33, which we cap.
	h := 1 + bits.TrailingZeros32(rand.Uint32())
	if h > maxHeight {
		return maxHeight
	}
	return h
}
