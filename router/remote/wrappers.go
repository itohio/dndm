package remote

import (
	"context"
	"log/slog"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/network"
	"github.com/itohio/dndm/router"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
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
	*router.LocalIntent
	log    *slog.Logger
	remote network.Remote
	cfg    *types.Intent
	wd     *Watchdog
}

type LocalIntent struct {
	*router.LocalIntent
	log    *slog.Logger
	remote network.Remote
}

func wrapLocalIntent(log *slog.Logger, remote network.Remote) router.IntentWrapperFunc {
	return func(ii router.IntentInternal) (router.IntentInternal, error) {
		li, ok := ii.(*router.LocalIntent)
		if !ok {
			return nil, errors.ErrLocalIntent
		}

		ret := &LocalIntent{
			LocalIntent: li,
			log:         log,
			remote:      remote,
		}

		ctx, cancel := context.WithTimeout(li.Ctx(), time.Second)
		err := remote.Write(ctx, li.Route(), &types.Intent{
			Route:    li.Route().ID(),
			Ttl:      uint64(time.Minute),
			Register: true,
		})
		cancel()
		log.Info("LocalIntent", "send", "Intent.Register", "err", err)
		return ret, err
	}
}

func (i *LocalIntent) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	werr := i.remote.Write(ctx, i.LocalIntent.Route(), &types.Intent{
		Route:    i.LocalIntent.Route().ID(),
		Register: false,
	})
	cancel()
	i.log.Info("LocalIntent", "send", "Intent.Close", "err", werr)
	cerr := i.LocalIntent.Close()
	return errors.Join(werr, cerr)
}

func (i *LocalIntent) Send(ctx context.Context, msg proto.Message) error {
	return i.LocalIntent.Send(ctx, msg)
}
func (i *LocalIntent) Link(c chan<- proto.Message) {
	i.LocalIntent.Link(c)
}

// wrapRemoteIntent returns an intent wrapper that embeds remote
// wrap command and local intent into a RemoteIntent. It will start a watchdog timer
// that when expired will close the intent.
func wrapRemoteIntent(log *slog.Logger, remote network.Remote, ri *types.Intent) router.IntentWrapperFunc {
	return func(ii router.IntentInternal) (router.IntentInternal, error) {
		li, ok := ii.(*router.LocalIntent)
		if !ok {
			return nil, errors.ErrLocalIntent
		}

		ret := &RemoteIntent{
			LocalIntent: li,
			log:         log,
			remote:      remote,
			cfg:         ri,
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
	i.log.Info("RemoteIntent close")
	i.wd.Reset()
	return i.LocalIntent.Close()
}

func (i *RemoteIntent) Link(c chan<- proto.Message) {
	i.wd.Reset()
	i.LocalIntent.Link(c)
}

type RemoteInterest struct {
	*router.LocalInterest
	log    *slog.Logger
	remote network.Remote
	cfg    *types.Interest
	wd     *Watchdog
}

type LocalInterest struct {
	*router.LocalInterest
	log    *slog.Logger
	remote network.Remote
}

func wrapLocalInterest(log *slog.Logger, remote network.Remote) router.InterestWrapperFunc {
	return func(ii router.InterestInternal) (router.InterestInternal, error) {
		li, ok := ii.(*router.LocalInterest)
		if !ok {
			return nil, errors.ErrLocalIntent
		}
		ret := &LocalInterest{
			LocalInterest: li,
			log:           log,
			remote:        remote,
		}

		ctx, cancel := context.WithTimeout(li.Ctx(), time.Second)
		err := remote.Write(ctx, li.Route(), &types.Interest{
			Route:    li.Route().ID(),
			Ttl:      uint64(time.Minute),
			Register: true,
		})
		cancel()
		log.Info("LocalInterest", "send", "Intent.Register", "err", err)

		return ret, err
	}
}

func (i *LocalInterest) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	werr := i.remote.Write(ctx, i.LocalInterest.Route(), &types.Interest{
		Route:    i.LocalInterest.Route().ID(),
		Register: false,
	})
	cancel()
	i.log.Info("LocalInterest", "send", "Intent.Register", "err", werr)
	cerr := i.LocalInterest.Close()
	return errors.Join(werr, cerr)
}

func wrapRemoteInterest(log *slog.Logger, remote network.Remote, ri *types.Interest) router.InterestWrapperFunc {
	return func(ii router.InterestInternal) (router.InterestInternal, error) {
		li, ok := ii.(*router.LocalInterest)
		if !ok {
			return nil, errors.ErrLocalIntent
		}
		ret := &RemoteInterest{
			LocalInterest: li,
			log:           log,
			remote:        remote,
			cfg:           ri,
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
	i.log.Info("RemoteInterest close")
	i.wd.Reset()
	return i.LocalInterest.Close()
}
