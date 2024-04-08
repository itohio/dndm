package pipe

import (
	reflect "reflect"
	"time"

	"github.com/itohio/dndm/errors"
	"github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/routers/pipe/types"
	"google.golang.org/protobuf/proto"
)

func (t *Transport) messageHandler() {
	defer t.wg.Done()
	for {
		select {
		case <-t.ctx.Done():
			return
		default:
		}

		hdr, msg, err := t.readAndParseMessage()
		if err != nil {
			t.log.Error("decode failed", "err", err)
			continue
		}

		err = t.handleMessage(hdr, msg)
		if err != nil {
			t.log.Error("handle failed", "err", err)
			continue
		}
	}
}

func (t *Transport) readAndParseMessage() (*types.Header, proto.Message, error) {
	data, err := ReadMessage(t.reader)
	if err != nil {
		return nil, nil, err
	}
	defer buffers.Put(data)

	// TODO: optimize
	t.mu.Lock()
	interests := make(map[string]routers.Route, len(t.interests))
	for k, v := range t.interests {
		interests[k] = v.Route()
	}
	t.mu.Unlock()

	hdr, msg, err := DecodeMessage(data, interests)
	if err != nil {
		return nil, nil, err
	}
	return hdr, msg, nil
}

func (t *Transport) handleMessage(hdr *types.Header, msg proto.Message) error {
	switch hdr.Type {
	case types.Type_HANDSHAKE:
		return t.handleHandshake(hdr, msg)
	case types.Type_PING:
		return t.handlePing(hdr, msg)
	case types.Type_PONG:
		return t.handlePong(hdr, msg)
	}
	return errors.ErrInvalidRoute
}

func (t *Transport) sendMessage(msg proto.Message, route routers.Route) error {
	buf, err := EncodeMessage(msg, t.name, route)
	if err != nil {
		t.log.Error("failed encoding", "what", reflect.TypeOf(msg), "err", err)
		return err
	}
	defer buffers.Put(buf)
	n, err := t.writer.Write(buf)
	if n != len(buf) || err != nil {
		t.log.Error("failed writing bytes", "what", reflect.TypeOf(msg), "err", err, "n", n, "len(buf)", len(buf))
	}
	return err
}

func (t *Transport) messageSender(d time.Duration) {
	defer t.wg.Done()
	ticker := time.NewTicker(d)

	t.sendMessage(
		&types.Handshake{
			Id: t.name,
		},
		routers.Route{},
	)

	for {
		select {
		case <-t.ctx.Done():
			return
		case buf := <-t.messageQueue:
			n, err := t.writer.Write(buf)
			if n != len(buf) || err != nil {
				t.log.Error("failed writing bytes", "what", "msg", "err", err, "n", n, "len(buf)", len(buf))
			}
			buffers.Put(buf)
		case <-ticker.C:
			ping := &types.Ping{
				Id:        t.pingNonce.Add(1),
				Timestamp: uint64(time.Now().UnixNano()),
			}
			t.sendMessage(
				ping,
				routers.Route{},
			)
			t.pingMu.Lock()
			t.pingRing.Value = ping
			t.pingRing = t.pingRing.Next()
			t.pingMu.Unlock()
		}
	}
}

func (t *Transport) handleHandshake(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Handshake)
	if !ok {
		return errors.ErrInvalidType
	}

	if t.remote == nil {
		t.remote = msg
		return nil
	}
	if t.remote.Id != msg.Id {
		t.log.Error("bad handshake", "want", t.remote.Id, "got", msg.Id)
	}
	t.remote = msg
	return nil
}

func (t *Transport) handlePeers(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Peers)
	if !ok {
		return errors.ErrInvalidType
	}
	_ = msg
	return nil
}

func (t *Transport) handlePing(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Ping)
	if !ok {
		return errors.ErrInvalidType
	}
	pong := &types.Pong{
		Id:            msg.Id,
		PingTimestamp: msg.Timestamp,
		Timestamp:     uint64(time.Now().UnixNano()),
		Payload:       msg.Payload,
	}

	t.sendMessage(
		pong,
		routers.Route{},
	)
	t.pingMu.Lock()
	defer t.pingMu.Unlock()
	t.pongRing.Value = pong
	t.pongRing = t.pingRing.Next()

	return nil
}

func (t *Transport) handlePong(hdr *types.Header, m proto.Message) error {
	msg, ok := m.(*types.Pong)
	if !ok {
		return errors.ErrInvalidType
	}
	now := uint64(time.Now().UnixNano())

	t.pingMu.Lock()
	defer t.pingMu.Unlock()

	t.pingRing.Do(func(a any) {
		p, ok := a.(*types.Ping)
		if !ok {
			return
		}
		if p.Id != msg.Id {
			return
		}

		// TODO
		rtt := now - p.Timestamp
		_ = rtt
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
	_ = msg
	return nil
}

func (t *Transport) handleInterest(hdr *types.Header, m proto.Message) error {
	if m == nil {
		return errors.ErrInvalidType
	}
	msg, ok := m.(*types.Interest)
	if !ok {
		return errors.ErrInvalidType
	}
	_ = msg
	return nil
}
