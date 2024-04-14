package mesh

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/itohio/dndm/dialers"
	"github.com/itohio/dndm/errors"
	p2ptypes "github.com/itohio/dndm/types/p2p"
)

type AddrbookEntry struct {
	sync.Mutex

	Peer              dialers.Peer
	MaxAttempts       int
	DefaultBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64

	Attempts    int
	Failed      int
	LastSuccess time.Time
	Backoff     time.Duration
}

func NewAddrbookEntry(p *p2ptypes.AddrbookEntry) *AddrbookEntry {
	ret := &AddrbookEntry{
		Peer:              errors.Must(dialers.PeerFromString(p.Peer)),
		MaxAttempts:       int(p.MaxAttempts),
		DefaultBackoff:    time.Duration(p.DefaultBackoff),
		MaxBackoff:        time.Duration(p.MaxBackoff),
		BackoffMultiplier: float64(p.BackoffMultiplier),
		Attempts:          int(p.Attempts),
		Failed:            int(p.FailedAttempts),
		LastSuccess:       time.Unix(0, int64(p.LastSuccess)),
		Backoff:           time.Duration(p.Backoff),
	}
	if ret.BackoffMultiplier < .1 {
		ret.BackoffMultiplier = .1
	}
	if ret.DefaultBackoff < time.Millisecond {
		ret.DefaultBackoff = time.Millisecond
	}
	if ret.MaxBackoff < time.Second {
		ret.MaxBackoff = time.Second
	}
	if ret.Backoff < ret.DefaultBackoff {
		ret.Backoff = ret.DefaultBackoff
	}
	return ret
}

func (a *AddrbookEntry) Dial(ctx context.Context, log *slog.Logger, dialer dialers.Dialer, q chan<- *AddrbookEntry) (io.ReadWriter, error) {
	rw, err := dialer.Dial(ctx, a.Peer)
	a.Lock()
	defer a.Unlock()
	if err == nil {
		a.Attempts++
		a.Failed = 0
		a.LastSuccess = time.Now()
		a.Backoff = a.DefaultBackoff
		return rw, nil
	}

	a.Failed++
	if a.Failed > a.MaxAttempts {
		log.Info("Max attempts", "num", a.Failed, "peer", a.Peer)
		return nil, errors.ErrNotFound
	}
	if a.Backoff < a.DefaultBackoff {
		a.Backoff = a.DefaultBackoff
	}

	a.Backoff += time.Duration(float64(time.Second) * (a.Backoff.Seconds() * a.BackoffMultiplier))
	if a.Backoff > a.MaxBackoff {
		a.Backoff = a.MaxBackoff
	}

	t := time.NewTimer(a.Backoff)
	go func() {
		select {
		case <-ctx.Done():
		case <-t.C:
			log.Info("Retry", "num", a.Failed, "peer", a.Peer, "backoff", a.Backoff)
		}

		select {
		case <-ctx.Done():
		case q <- a:
		}
	}()

	return nil, errors.ErrRetry
}
