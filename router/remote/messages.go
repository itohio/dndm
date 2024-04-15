package remote

import (
	"bytes"
	"context"
	"crypto/rand"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/router"
	types "github.com/itohio/dndm/types/core"
	"google.golang.org/protobuf/proto"
)

type Ping struct {
	timestamp uint64
	payload   []byte
}

type Pong struct {
	timestamp     uint64
	pingTimestamp uint64
	payload       []byte
}

func (t *Remote) messageHandler() {
	t.Log.Info("messageHandler", "loop", "START")
	defer t.Log.Info("messageHandler", "loop", "STOP")

	defer t.wg.Done()
	for {
		select {
		case <-t.Ctx.Done():
			return
		default:
		}

		hdr, msg, err := t.remote.Read(t.Ctx)
		if err != nil {
			t.Log.Error("remote.Read", "err", err)
			t.Close()
			return
		}
		// t.log.Info("got msg", "route", hdr.Route, "hdr.type", hdr.Type, "type", reflect.TypeOf(msg))
		prevNonce := t.nonce.Load()
		duration := time.Duration(hdr.Timestamp) - time.Duration(prevNonce)
		if duration <= 0 {
			t.Log.Warn("hdr.Timestamp", "hdr", hdr.Timestamp, "nonce", prevNonce, "duration", duration)
		} else {
			t.nonce.Store(hdr.Timestamp)
		}

		err = t.handleMessage(hdr, msg)
		if err != nil {
			t.Log.Error("handleMessage", "err", err)
		}
		// TODO: Awkward if handler wants to override result they must set hdr.WantResult = false
		if hdr.Type != types.Type_MESSAGE && hdr.WantResult {
			result := &types.Result{
				Nonce: hdr.Timestamp,
			}
			if err != nil {
				result.Error = 1
				result.Description = err.Error()
			}
			err := t.remote.Write(t.Ctx, router.Route{}, result)
			if err != nil {
				t.Log.Error("write result", "err", err, "result", result)
			}
		}
	}
}

func (t *Remote) handleMessage(hdr *types.Header, msg proto.Message) error {
	switch hdr.Type {
	case types.Type_HANDSHAKE:
		return nil
	case types.Type_PEERS:
		return nil
	case types.Type_ADDRBOOK:
		return nil
	case types.Type_PING:
		return t.handlePing(hdr, msg)
	case types.Type_PONG:
		return t.handlePong(hdr, msg)
	case types.Type_INTENT:
		return t.handleIntent(hdr, msg)
	case types.Type_INTENTS:
		return t.handleIntents(hdr, msg)
	case types.Type_INTEREST:
		return t.handleInterest(hdr, msg)
	case types.Type_INTERESTS:
		return t.handleInterests(hdr, msg)
	case types.Type_MESSAGE:
		return t.handleMsg(hdr, msg)
	case types.Type_RESULT:
		return t.handleResult(hdr, msg)
	}
	return errors.ErrInvalidRoute
}

func (t *Remote) messageSender(d time.Duration) {
	defer t.wg.Done()
	ticker := time.NewTicker(d)

	t.Log.Info("Remote.Pinging")
	for {
		select {
		case <-t.Ctx.Done():
			return
		case <-ticker.C:
			ping := &types.Ping{
				Payload: make([]byte, 1024),
			}
			rand.Read(ping.Payload)
			err := t.remote.Write(
				t.Ctx,
				router.Route{},
				ping,
			)
			t.Log.Info("Remote.Ping", "send", err)
			t.pingMu.Lock()
			t.pingRing.Value = &Ping{
				timestamp: uint64(time.Now().UnixNano()),
				payload:   ping.Payload,
			}
			t.pingRing = t.pingRing.Next()
			t.pingMu.Unlock()
		}
	}
}

func (t *Remote) handleResult(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Result)
	if !ok {
		return errors.ErrInvalidType
	}

	if msg.Error == 0 {
		return nil
	}

	t.Log.Error("Result", "err", msg.Error, "description", msg.Description)
	return nil
}

func (t *Remote) handlePing(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Ping)
	if !ok {
		return errors.ErrInvalidType
	}
	if len(msg.Payload) < 16 {
		return errors.ErrNotEnoughBytes
	}
	pong := &types.Pong{
		ReceiveTimestamp: hdr.ReceiveTimestamp,
		PingTimestamp:    hdr.Timestamp,
		Payload:          msg.Payload,
	}

	err := t.remote.Write(
		t.Ctx,
		router.Route{},
		pong,
	)
	t.Log.Info("Remote.Pong", "send", err)
	t.pingMu.Lock()
	t.pongRing.Value = &Pong{
		timestamp:     hdr.ReceiveTimestamp,
		pingTimestamp: hdr.Timestamp,
		payload:       msg.Payload,
	}
	t.pongRing = t.pingRing.Next()
	t.pingMu.Unlock()

	return nil
}

func (t *Remote) handlePong(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Pong)
	if !ok {
		return errors.ErrInvalidType
	}
	t.pingMu.Lock()
	defer t.pingMu.Unlock()

	t.pingRing.Do(func(a any) {
		p, ok := a.(*Ping)
		if !ok {
			return
		}
		if !bytes.Equal(p.payload, msg.Payload) {
			return
		}

		// TODO: now - when sent
		rtt := time.Duration(hdr.ReceiveTimestamp - p.timestamp)
		// now - when sent as reported by remote
		rtt1 := time.Duration(hdr.ReceiveTimestamp - msg.PingTimestamp)
		_ = rtt
		_ = rtt1
		// too much difference may indicate non-compliant remote
		// also, msg.PingTimestamp, hdr.ReceiveTimestamp are local times
		// while hdr.Timestamp, and msg.ReceiveTimestamp are remote times
		// FIXME: this is utterly confusing. Need better names.
		t.Log.Info("Remote.Ping-Pong", "rtt", rtt, "rtt1", rtt1, "name", t.Name(), "peer", t.remote.RemotePeer())
	})

	return nil
}

func (t *Remote) handleIntent(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Intent)
	if !ok {
		return errors.ErrInvalidType
	}
	return t.handleRemoteIntent(msg)
}

func (t *Remote) handleIntents(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Intents)
	if !ok {
		return errors.ErrInvalidType
	}
	for _, i := range msg.Intents {
		if err := t.handleRemoteIntent(i); err != nil {
			return err
		}
	}
	return nil
}

func (t *Remote) handleRemoteIntent(msg *types.Intent) error {
	route, err := router.NewRouteFromRoute(msg.Route)
	if err != nil {
		return err
	}

	if !msg.Register {
		return t.handleUnregisterIntent(route, msg)
	}

	intent, err := t.publish(route, msg)
	if err != nil {
		return err
	}
	if _, ok := intent.(*LocalIntent); ok {
		return errors.ErrLocalIntent
	}

	return nil
}

func (t *Remote) handleUnregisterIntent(route router.Route, m *types.Intent) error {
	intent, ok := t.linker.Intent(route)
	if !ok {
		return errors.ErrNoIntent
	}

	if intent, ok := intent.(*RemoteIntent); !ok {
		return errors.ErrForbidden
	} else if intent.cfg == nil || intent.cfg.Route != m.Route {
		return errors.ErrForbidden
	}

	return intent.Close()
}

func (t *Remote) handleInterest(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Interest)
	if !ok {
		return errors.ErrInvalidType
	}
	return t.handleRemoteInterest(msg)
}

func (t *Remote) handleInterests(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Interests)
	if !ok {
		return errors.ErrInvalidType
	}
	for _, i := range msg.Interests {
		if err := t.handleRemoteInterest(i); err != nil {
			return err
		}
	}
	return nil
}

func (t *Remote) handleRemoteInterest(msg *types.Interest) error {
	route, err := router.NewRouteFromRoute(msg.Route)
	if err != nil {
		return err
	}

	if !msg.Register {
		return t.handleUnregisterInterest(route, msg)
	}

	interest, err := t.subscribe(route, msg)
	if err != nil {
		return err
	}

	if _, ok := interest.(*LocalInterest); ok {
		return errors.ErrLocalInterest
	}

	t.AddCallback(interest, t)

	return nil
}

func (t *Remote) handleUnregisterInterest(route router.Route, m *types.Interest) error {
	interest, ok := t.linker.Interest(route)
	if !ok {
		return errors.ErrNoIntent
	}

	if interest, ok := interest.(*RemoteInterest); !ok {
		return errors.ErrForbidden
	} else if interest.cfg == nil || interest.cfg.Route != m.Route {
		return errors.ErrForbidden
	}

	return interest.Close()
}

func (t *Remote) handleMsg(hdr *types.Header, m proto.Message) error {
	route, err := router.NewRouteFromRoute(hdr.Route)
	if err != nil {
		return err
	}

	intent, ok := t.linker.Intent(route)
	if !ok {
		return errors.ErrNoIntent
	}

	ctx, cancel := context.WithTimeout(t.Ctx, t.timeout)
	err = intent.Send(ctx, m)
	cancel()
	return err
}
