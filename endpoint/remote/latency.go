package remote

import (
	"bytes"
	"container/ring"
	"crypto/rand"
	sync "sync"
	"time"

	types "github.com/itohio/dndm/types/core"
)

type LatencyTracker struct {
	mu       sync.Mutex
	pingIdx  int
	rtt1     []time.Duration
	rtt2     []time.Duration
	pingRing *ring.Ring
	pongRing *ring.Ring
}

func NewLatencyTracker(size int) *LatencyTracker {
	return &LatencyTracker{
		rtt1:     make([]time.Duration, size),
		rtt2:     make([]time.Duration, size),
		pingRing: ring.New(3),
		pongRing: ring.New(3),
	}
}

func (l *LatencyTracker) ObservePong(hdr *types.Header, msg *types.Pong) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.pingRing.Do(func(a any) {
		p, ok := a.(*Ping)
		if !ok {
			return
		}
		if !bytes.Equal(p.payload, msg.Payload) {
			return
		}

		// TODO: now - when sent
		rtt1 := time.Duration(hdr.ReceiveTimestamp - p.timestamp)
		// now - when sent as reported by remote
		rtt2 := time.Duration(hdr.ReceiveTimestamp - msg.PingTimestamp)

		l.rtt1[l.pingIdx] = rtt1
		l.rtt2[l.pingIdx] = rtt2
		l.pingIdx = (l.pingIdx + 1) % len(l.rtt1)

		// too much difference may indicate non-compliant remote
		// also, msg.PingTimestamp, hdr.ReceiveTimestamp are local times
		// while hdr.Timestamp, and msg.ReceiveTimestamp are remote times
		// FIXME: this is utterly confusing. Need better names.
		// t.Log.Info("Remote.Ping-Pong", "rtt", rtt, "rtt1", rtt1, "name", t.Name(), "peer", t.conn.Remote())
	})
}

func (l *LatencyTracker) MakePing(size int) *types.Ping {
	ping := &types.Ping{
		Payload: make([]byte, size),
	}
	rand.Read(ping.Payload)
	l.mu.Lock()
	l.pingRing.Value = &Ping{
		timestamp: uint64(time.Now().UnixNano()),
		payload:   ping.Payload,
	}
	l.pingRing = l.pingRing.Next()
	l.mu.Unlock()
	return ping
}

func (l *LatencyTracker) MakePong(hdr *types.Header, ping *types.Ping) *types.Pong {
	pong := &types.Pong{
		ReceiveTimestamp: hdr.ReceiveTimestamp,
		PingTimestamp:    hdr.Timestamp,
		Payload:          ping.Payload,
	}

	l.mu.Lock()
	l.pongRing.Value = &Pong{
		timestamp:     hdr.ReceiveTimestamp,
		pingTimestamp: hdr.Timestamp,
		payload:       ping.Payload,
	}
	l.pongRing = l.pongRing.Next()
	l.mu.Unlock()
	return pong
}
