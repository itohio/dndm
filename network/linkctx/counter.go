package linkctx

// AEAD counter low bits (design link-context-wire.md §4, verify DND-R7 F2).
//
// The counter is per direction, per session, and starts at 1. A frame carries only its low
// bits: 7 on profiles where every byte counts (classic CAN, CRSF), 14 elsewhere. The
// receiver rebuilds the full counter from the highest one it has accepted, choosing the
// value with the same low bits that is closest to it: ahead (frames lost) or behind (a
// late frame the replay window can still accept). That is unambiguous within half the
// window, up to 63 frames at 7 bits and 8191 at 14 bits; a tie at exactly half resolves
// ahead. Beyond that the rebuilt counter is wrong and the tag check fails, so a
// mis-rebuilt counter can never be accepted (tags: DND-009).

// Counter widths per profile family.
const (
	CounterBitsSmall = 7  // classic CAN, CRSF
	CounterBitsWide  = 14 // every other profile
)

// LowBits returns the low bits of ctr that a frame carries.
func LowBits(ctr uint64, bits uint) uint32 {
	return uint32(ctr & (1<<bits - 1))
}

// RebuildCounter returns the full counter with low bits low that is closest to highest,
// the highest counter accepted so far.
func RebuildCounter(highest uint64, low uint32, bits uint) uint64 {
	window := uint64(1) << bits
	half := window / 2
	c := highest&^(window-1) | uint64(low)
	switch {
	case c > highest && c-highest > half:
		if c >= window { // near zero there is nothing behind
			c -= window
		}
	case c < highest && highest-c >= half:
		c += window
	}
	return c
}
