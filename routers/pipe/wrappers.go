package pipe

import (
	"github.com/itohio/dndm/routers/direct"
	types "github.com/itohio/dndm/routers/pipe/types"
)

type RemoteIntent struct {
	*direct.Intent
	remote *types.Intent
}

func wrapIntent(pi *direct.Intent, remote *types.Intent) *RemoteIntent {
	ret := &RemoteIntent{
		Intent: pi,
		remote: remote,
	}
	return ret
}

type RemoteInterest struct {
	*direct.Interest
	remote *types.Interest
}

func wrapInterest(pi *direct.Interest, remote *types.Interest) *RemoteInterest {
	ret := &RemoteInterest{
		Interest: pi,
		remote:   remote,
	}

	// TODO: run a message loop

	return ret
}
