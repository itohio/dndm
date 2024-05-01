package errors

import "errors"

var (
	Is     = errors.Is
	Join   = errors.Join
	As     = errors.As
	Unwrap = errors.Unwrap
	New    = errors.New

	ErrClosed             = errors.New("closed")
	ErrNotEnoughBytes     = errors.New("not enough bytes")
	ErrNotFound           = errors.New("not found")
	ErrRetry              = errors.New("will retry")
	ErrNoInterest         = errors.New("no interest")
	ErrNoIntent           = errors.New("no intent")
	ErrForbidden          = errors.New("forbidden")
	ErrBadArgument        = errors.New("bad argument")
	ErrDuplicate          = errors.New("duplicate interest")
	ErrInvalidEndpoint    = errors.New("invalid endpoint")
	ErrInvalidRoute       = errors.New("invalid route")
	ErrInvalidRoutePrefix = errors.New("invalid route prefix")
	ErrInvalidInterest    = errors.New("invalid interest")
	ErrInvalidType        = errors.New("invalid type")
	ErrTooFast            = errors.New("too fast")
	ErrLocalInterest      = errors.New("local interest")
	ErrLocalIntent        = errors.New("local intent")
	ErrRemoteInterest     = errors.New("remote interest")
	ErrRemoteIntent       = errors.New("remote intent")

	ErrStopBits = errors.New("stop bits")
)

func Must[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}
