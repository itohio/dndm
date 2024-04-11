package remote

import (
	"context"
	"log/slog"
	"time"

	"github.com/itohio/dndm/errors"
	routers "github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const DefaultTimeToLive = time.Hour * 24 * 30 * 365

type Watchdog struct {
	*time.Timer
	ttl time.Duration
}

func newWatchdog(ttlU uint64) *Watchdog {
	ttl := DefaultTimeToLive
	if ttlU > uint64(time.Millisecond) {
		ttl = time.Duration(ttlU)
	}
	return &Watchdog{
		Timer: time.NewTimer(ttl),
		ttl:   ttl,
	}
}

func (wd *Watchdog) Reset() {
	wd.Timer.Reset(wd.ttl)
}

type RemoteIntent struct {
	*routers.LocalIntent
	remote *types.Intent
	wd     *Watchdog
}

// wrapIntent returns an intent wrapper that embeds remote
// wrap command and local intent into a RemoteIntent. It will start a watchdog timer
// that when expired will close the intent.
func wrapIntent(log *slog.Logger, ri *types.Intent) routers.IntentWrapperFunc {
	return func(ii routers.IntentInternal) (routers.IntentInternal, error) {
		li, ok := ii.(*routers.LocalIntent)
		if !ok {
			return nil, errors.ErrLocalIntent
		}

		ret := &RemoteIntent{
			LocalIntent: li,
			remote:      ri,
			wd:          newWatchdog(ri.Ttl),
		}

		go func() {
			defer ret.Close()
			for {
				select {
				case <-li.Ctx().Done():
					return
				case <-ret.wd.C:
					log.Info("ttl reached", "ttl", ret.wd.ttl, "intent", li.Route())
					return
				}
			}
		}()

		return ret, nil
	}
}

func (i *RemoteIntent) Send(ctx context.Context, msg proto.Message) error {
	i.wd.Reset()
	return i.LocalIntent.Send(ctx, msg)
}

func (i *RemoteIntent) Close() error {
	i.wd.Reset()
	return i.LocalIntent.Close()
}

func (i *RemoteIntent) Link(c chan<- protoreflect.ProtoMessage) {
	i.wd.Reset()
	i.LocalIntent.Link(c)
}

type RemoteInterest struct {
	*routers.LocalInterest
	remote *types.Interest
	wd     *Watchdog
}

func wrapInterest(log *slog.Logger, remote Remote, ri *types.Interest) routers.InterestWrapperFunc {
	return func(ii routers.InterestInternal) (routers.InterestInternal, error) {
		li, ok := ii.(*routers.LocalInterest)
		if !ok {
			return nil, errors.ErrLocalIntent
		}
		ret := &RemoteInterest{
			LocalInterest: li,
			remote:        ri,
			wd:            newWatchdog(ri.Ttl),
		}

		go func() {
			defer li.Close()
			for {
				select {
				case <-li.Ctx().Done():
					return
				case <-ret.wd.C:
					log.Info("ttl reached", "ttl", ret.wd.ttl, "interest", li.Route())
					return
				case m := <-li.C():
					if err := remote.Write(li.Ctx(), li.Route(), m); err != nil {
						log.Error("remote interest write", "err", err, "route", li.Route())
					}
					ret.wd.Reset()
				}
			}
		}()

		return ret, nil
	}
}

func (i *RemoteInterest) Close() error {
	i.wd.Reset()
	return i.LocalInterest.Close()
}
