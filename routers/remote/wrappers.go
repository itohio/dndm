package remote

import (
	"github.com/itohio/dndm/errors"
	routers "github.com/itohio/dndm/routers"
	types "github.com/itohio/dndm/types/core"
)

type RemoteIntent struct {
	*routers.LocalIntent
	remote *types.Intent
}

func wrapIntent(remote *types.Intent) routers.IntentWrapperFunc {
	return func(ii routers.IntentInternal) (routers.IntentInternal, error) {
		li, ok := ii.(*routers.LocalIntent)
		if !ok {
			return nil, errors.ErrLocalIntent
		}
		ret := &RemoteIntent{
			LocalIntent: li,
			remote:      remote,
		}
		return ret, nil
	}
}

type RemoteInterest struct {
	*routers.LocalInterest
	remote *types.Interest
}

func wrapInterest(remote *types.Interest) routers.InterestWrapperFunc {
	return func(ii routers.InterestInternal) (routers.InterestInternal, error) {
		li, ok := ii.(*routers.LocalInterest)
		if !ok {
			return nil, errors.ErrLocalIntent
		}
		ret := &RemoteInterest{
			LocalInterest: li,
			remote:        remote,
		}

		return ret, nil
	}
}
