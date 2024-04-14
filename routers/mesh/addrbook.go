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
	BackOffMultiplier float64

	Attempts    int
	Failed      int
	LastSuccess time.Time
	BackOff     time.Duration
}

func NewAddrbookEntry(p *p2ptypes.AddrbookEntry) *AddrbookEntry {
	return &AddrbookEntry{
		Peer:              errors.Must(dialers.PeerFromString(p.Peer)),
		MaxAttempts:       int(p.MaxAttempts),
		DefaultBackoff:    time.Duration(p.DefaultBackoff),
		MaxBackoff:        time.Duration(p.MaxBackoff),
		BackOffMultiplier: float64(p.BackoffMultiplier),
		Attempts:          int(p.Attempts),
		Failed:            int(p.FailedAttempts),
		LastSuccess:       time.Unix(0, int64(p.LastSuccess)),
		BackOff:           time.Duration(p.Backoff),
	}
}

func (a *AddrbookEntry) Dial(ctx context.Context, log *slog.Logger, dialer dialers.Dialer, q chan<- *AddrbookEntry) (io.ReadWriter, error) {
	rw, err := dialer.Dial(ctx, a.Peer)
	a.Lock()
	defer a.Unlock()
	if err == nil {
		a.Attempts++
		a.Failed = 0
		a.LastSuccess = time.Now()
		a.BackOff = a.DefaultBackoff
		return rw, nil
	}

	a.Failed++
	if a.Failed > a.MaxAttempts {
		log.Info("Max attempts", "num", a.Failed, "peer", a.Peer)
		return nil, errors.ErrNotFound
	}

	a.BackOff += time.Duration(float64(time.Second) * (a.BackOff.Seconds() * a.BackOffMultiplier))
	if a.BackOff > a.MaxBackoff {
		a.BackOff = a.MaxBackoff
	}

	t := time.NewTimer(a.BackOff)
	go func() {
		select {
		case <-ctx.Done():
		case <-t.C:
		}

		select {
		case <-ctx.Done():
		case q <- a:
		}
	}()

	return nil, errors.ErrRetry
}
