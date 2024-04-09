package p2p

import (
	routers "github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
)

type RemoteIntent struct {
	*routers.LocalIntent
	remote *types.Intent
}

func wrapIntent(pi *routers.LocalIntent, remote *types.Intent) *RemoteIntent {
	ret := &RemoteIntent{
		LocalIntent: pi,
		remote:      remote,
	}
	return ret
}

type RemoteInterest struct {
	*routers.LocalInterest
	remote *types.Interest
}

func wrapInterest(pi *routers.LocalInterest, remote *types.Interest) *RemoteInterest {
	ret := &RemoteInterest{
		LocalInterest: pi,
		remote:        remote,
	}

	// TODO: run a message loop

	return ret
}
