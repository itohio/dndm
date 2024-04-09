package codec

import (
	"fmt"
	reflect "reflect"
	sync "sync"

	types "github.com/itohio/dndm/types/core"
	p2ptypes "github.com/itohio/dndm/types/p2p"
	"google.golang.org/protobuf/proto"
)

var (
	knownTypes         = map[types.Type]reflect.Type{}
	knownTypesReversed = map[reflect.Type]types.Type{}
	mu                 sync.Mutex
)

func init() {
	RegisterType(types.Type_NOTIFY_INTENT, &types.NotifyIntent{})
	RegisterType(types.Type_RESULT, &types.Result{})
	RegisterType(types.Type_INTENT, &types.Intent{})
	RegisterType(types.Type_INTENTS, &types.Intents{})
	RegisterType(types.Type_INTEREST, &types.Interest{})
	RegisterType(types.Type_INTERESTS, &types.Interests{})
	RegisterType(types.Type_HANDSHAKE, &p2ptypes.Handshake{})
	RegisterType(types.Type_PEERS, &p2ptypes.Peers{})
	RegisterType(types.Type_PING, &types.Ping{})
	RegisterType(types.Type_PONG, &types.Pong{})
}

func RegisterType(t types.Type, msg proto.Message) {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := knownTypes[t]; ok {
		panic(fmt.Errorf("type already registered: %v", t))
	}
	tt := reflect.TypeOf(msg).Elem()
	knownTypes[t] = tt
	knownTypesReversed[tt] = t
}

func resolveType(msg proto.Message) types.Type {
	tt := reflect.TypeOf(msg).Elem()
	if t, ok := knownTypesReversed[tt]; ok {
		return t
	}
	return types.Type_MESSAGE
}
