package p2p

import (
	"bytes"
	"context"
	"crypto/rand"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
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

func (t *Transport) messageHandler() {
	defer t.wg.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		hdr, msg, err := t.dialer.Read(t.ctx)
		if err != nil {
			t.log.Error("decode failed", "err", err)
			continue
		}
		prevNonce := t.nonce.Load()
		if hdr.Timestamp < prevNonce {
			t.log.Error("nonce", "prev", prevNonce, "got", hdr.Timestamp)
			continue
		}
		t.nonce.Store(hdr.Timestamp)

		err = t.handleMessage(hdr, msg)
		if err != nil {
			t.log.Error("handleMessage", "err", err)
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
			err := t.dialer.Write(t.ctx, routers.Route{}, result)
			if err != nil {
				t.log.Error("write result", "err", err, "result", result)
			}
		}
	}
}

func (t *Transport) handleMessage(hdr *types.Header, msg proto.Message) error {
	switch hdr.Type {
	case types.Type_HANDSHAKE:
		return nil
	case types.Type_PEERS:
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

func (t *Transport) messageSender(d time.Duration) {
	defer t.wg.Done()
	ticker := time.NewTicker(d)

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			ping := &types.Ping{
				Payload: make([]byte, 1024),
			}
			rand.Read(ping.Payload)
			t.dialer.Write(
				t.ctx,
				routers.Route{},
				ping,
			)
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

func (t *Transport) handleResult(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Result)
	if !ok {
		return errors.ErrInvalidType
	}

	if msg.Error == 0 {
		return nil
	}

	t.log.Error("Result", "err", msg.Error, "description", msg.Description)
	return nil
}

func (t *Transport) handlePing(hdr *types.Header, m proto.Message) error {
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

	t.dialer.Write(
		t.ctx,
		routers.Route{},
		pong,
	)
	t.pingMu.Lock()
	defer t.pingMu.Unlock()
	t.pongRing.Value = &Pong{
		timestamp:     hdr.ReceiveTimestamp,
		pingTimestamp: hdr.Timestamp,
		payload:       msg.Payload,
	}
	t.pongRing = t.pingRing.Next()

	return nil
}

func (t *Transport) handlePong(hdr *types.Header, m proto.Message) error {
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
		rtt := hdr.ReceiveTimestamp - p.timestamp
		// now - when sent as reported by remote
		rtt1 := hdr.ReceiveTimestamp - msg.PingTimestamp
		_ = rtt
		_ = rtt1
		// too much difference may indicate non-compliant remote
		// also, msg.PingTimestamp, hdr.ReceiveTimestamp are local times
		// while hdr.Timestamp, and msg.ReceiveTimestamp are remote times
		// FIXME: this is utterly confusing. Need better names.
	})

	return nil
}

func (t *Transport) handleIntent(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Intent)
	if !ok {
		return errors.ErrInvalidType
	}
	return t.handleRemoteIntent(msg)
}

func (t *Transport) handleIntents(hdr *types.Header, m proto.Message) error {
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

func (t *Transport) handleRemoteIntent(msg *types.Intent) error {
	route, err := routers.NewRouteFromRoute(msg.Route)
	if err != nil {
		return err
	}

	if !msg.Register {
		return t.handleUnregisterIntent(route, msg)
	}

	_, err = t.publish(route, msg)
	if err != nil {
		return err
	}

	return nil
}

func (t *Transport) handleUnregisterIntent(route routers.Route, m *types.Intent) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	intent, ok := t.intents[route.ID()]
	if !ok {
		return errors.ErrNoIntent
	}

	if intent, ok := intent.(*RemoteIntent); !ok {
		return errors.ErrForbidden
	} else if intent.remote == nil || intent.remote.Route != m.Route {
		return errors.ErrForbidden
	}

	return intent.Close()
}

func (t *Transport) handleInterest(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Interest)
	if !ok {
		return errors.ErrInvalidType
	}
	return t.handleRemoteInterest(msg)
}

func (t *Transport) handleInterests(hdr *types.Header, m proto.Message) error {
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

func (t *Transport) handleRemoteInterest(msg *types.Interest) error {
	route, err := routers.NewRouteFromRoute(msg.Route)
	if err != nil {
		return err
	}

	if !msg.Register {
		return t.handleUnregisterInterest(route, msg)
	}

	intent, err := t.subscribe(route, msg)
	if err != nil {
		return err
	}
	t.addCallback(intent, t)

	return nil
}

func (t *Transport) handleUnregisterInterest(route routers.Route, m *types.Interest) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	interest, ok := t.interests[route.ID()]
	if !ok {
		return errors.ErrNoIntent
	}

	if interest, ok := interest.(*RemoteInterest); !ok {
		return errors.ErrForbidden
	} else if interest.remote == nil || interest.remote.Route != m.Route {
		return errors.ErrForbidden
	}

	return interest.Close()
}

func (t *Transport) handleMsg(hdr *types.Header, m proto.Message) error {
	route, err := routers.NewRouteFromRoute(hdr.Route)
	if err != nil {
		return err
	}

	// NOTE: Be aware of unreleased locks!
	t.mu.Lock()
	intent, ok := t.intents[route.ID()]
	if !ok {
		t.mu.Unlock()
		return errors.ErrNoIntent
	}
	t.mu.Unlock()

	ctx, cancel := context.WithTimeout(t.ctx, t.timeout)
	err = intent.Send(ctx, m)
	cancel()
	return err
}
