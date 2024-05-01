package remote

import (
	"context"
	"time"

	"github.com/itohio/dndm"
	"github.com/itohio/dndm/errors"
	types "github.com/itohio/dndm/types/core"
	p2ptypes "github.com/itohio/dndm/types/p2p"
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

func (t *Endpoint) messageHandler() {
	t.Log.Info("messageHandler", "loop", "START")
	defer t.Log.Info("messageHandler", "loop", "STOP")

	defer t.wg.Done()
	for {
		select {
		case <-t.Ctx().Done():
			return
		default:
		}

		hdr, msg, err := t.conn.Read(t.Ctx())
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
			err := t.conn.Write(t.Ctx(), dndm.EmptyRoute(), result)
			if err != nil {
				t.Log.Error("write result", "err", err, "result", result)
			}
		}
	}
}

func (t *Endpoint) handleMessage(hdr *types.Header, msg proto.Message) error {
	switch hdr.Type {
	case types.Type_HANDSHAKE:
		return t.handleHandshake(hdr, msg)
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

func (t *Endpoint) messageSender(d time.Duration) {
	defer t.wg.Done()
	ticker := time.NewTicker(d)

	t.Log.Info("Remote.Pinging")
	for {
		select {
		case <-t.Ctx().Done():
			return
		case <-ticker.C:
			ping := t.latency.MakePing(1024)
			err := t.conn.Write(t.Ctx(), dndm.EmptyRoute(), ping)
			t.Log.Debug("Remote.Ping", "send", err)
		}
	}
}

func (t *Endpoint) handleResult(hdr *types.Header, m proto.Message) error {
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

func (t *Endpoint) handlePing(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Ping)
	if !ok {
		return errors.ErrInvalidType
	}
	if len(msg.Payload) < 16 {
		return errors.ErrNotEnoughBytes
	}
	pong := t.latency.MakePong(hdr, msg)

	err := t.conn.Write(t.Ctx(), dndm.EmptyRoute(), pong)
	t.Log.Debug("Remote.Pong", "send", err)

	return nil
}

func (t *Endpoint) handlePong(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Pong)
	if !ok {
		return errors.ErrInvalidType
	}

	t.latency.ObservePong(hdr, msg)

	return nil
}

func (t *Endpoint) handleHandshake(hdr *types.Header, m proto.Message) error {
	handshake, ok := m.(*p2ptypes.Handshake)
	if !ok {
		return nil
	}
	t.Log.Info("HS", "handshake", handshake.Stage, "intents", len(handshake.Intents), "interests", len(handshake.Interests))
	if handshake.Stage != p2ptypes.HandshakeStage_FINAL {
		return nil
	}

	for _, i := range handshake.Intents {
		if err := t.handleRemoteIntent(i); err != nil {
			t.Log.Error("HS.intent", "err", err, "intent", i.Route)
			return err
		}
	}
	for _, i := range handshake.Interests {
		if err := t.handleRemoteInterest(i); err != nil {
			t.Log.Error("HS.interest", "err", err, "interest", i.Route)
			return err
		}
	}

	return nil
}

func (t *Endpoint) handleIntent(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Intent)
	if !ok {
		return errors.ErrInvalidType
	}
	return t.handleRemoteIntent(msg)
}

func (t *Endpoint) handleIntents(hdr *types.Header, m proto.Message) error {
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

func (t *Endpoint) handleRemoteIntent(msg *types.Intent) error {
	route, err := dndm.RouteFromString(msg.Route)
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

	t.OnAddIntent(intent, t)

	return nil
}

func (t *Endpoint) handleUnregisterIntent(route dndm.Route, m *types.Intent) error {
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

func (t *Endpoint) handleInterest(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Interest)
	if !ok {
		return errors.ErrInvalidType
	}
	return t.handleRemoteInterest(msg)
}

func (t *Endpoint) handleInterests(hdr *types.Header, m proto.Message) error {
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

func (t *Endpoint) handleRemoteInterest(msg *types.Interest) error {
	route, err := dndm.RouteFromString(msg.Route)
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

	t.OnAddInterest(interest, t)

	return nil
}

func (t *Endpoint) handleUnregisterInterest(route dndm.Route, m *types.Interest) error {
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

func (t *Endpoint) handleMsg(hdr *types.Header, m proto.Message) error {
	route, err := dndm.RouteFromString(hdr.Route)
	if err != nil {
		return err
	}

	intent, ok := t.linker.Intent(route)
	if !ok {
		return errors.ErrNoIntent
	}

	ctx, cancel := context.WithTimeout(t.Ctx(), t.timeout)
	err = intent.Send(ctx, m)
	cancel()
	return err
}
