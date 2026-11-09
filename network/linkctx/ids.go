package linkctx

import "time"

// idTable allocates the route ids this side sends DATA on (design §4, verify A3).
//
// Ids belong to a session: a new session starts empty, so nothing carries over between
// reconnects. The lowest free id from 1 is taken, so an id reaches 128 (5 B frames instead
// of 4 B) only with more than 127 routes bound at once. A freed id sits out a quarantine
// before reuse, so a late DATA frame for the old route cannot land on the new one on a
// reordering face; with a zero quarantine it is free at once.
type idTable struct {
	used       map[uint32]struct{}  // bound, binding or unbinding
	quarantine map[uint32]time.Time // freed id -> reusable from
}

func newIDTable() idTable {
	return idTable{used: make(map[uint32]struct{}), quarantine: make(map[uint32]time.Time)}
}

// alloc returns the lowest id that is neither used nor in quarantine at now. It fails when
// limit ids are already used.
func (t *idTable) alloc(limit int, now time.Time) (uint32, bool) {
	if len(t.used) >= limit {
		return 0, false
	}
	for id := uint32(1); ; id++ {
		if _, ok := t.used[id]; ok {
			continue
		}
		if until, ok := t.quarantine[id]; ok {
			if now.Before(until) {
				continue
			}
			delete(t.quarantine, id)
		}
		t.used[id] = struct{}{}
		return id, true
	}
}

// free returns an id that never carried DATA (its Bind was refused): no quarantine.
func (t *idTable) free(id uint32) {
	delete(t.used, id)
}

// release returns an id after UnbindAck. It is reusable from now+q.
func (t *idTable) release(id uint32, now time.Time, q time.Duration) {
	delete(t.used, id)
	if q > 0 {
		t.quarantine[id] = now.Add(q)
	}
}
