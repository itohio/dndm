package errors

import "errors"

var (
	Is     = errors.Is
	Join   = errors.Join
	As     = errors.As
	Unwrap = errors.Unwrap
	New    = errors.New

	ErrNotEnoughBytes  = errors.New("not enough bytes")
	ErrNoInterest      = errors.New("no interest")
	ErrNoIntent        = errors.New("no intent")
	ErrForbidden       = errors.New("forbidden")
	ErrInvalidRoute    = errors.New("invalid route")
	ErrBadArgument     = errors.New("bad argument")
	ErrDuplicate       = errors.New("duplicate interest")
	ErrInvalidInterest = errors.New("invalid interest")
	ErrInvalidType     = errors.New("invalid type")
	ErrTooFast         = errors.New("too fast")
	ErrLocalInterest   = errors.New("local interest")
	ErrLocalIntent     = errors.New("local intent")
	ErrRemoteInterest  = errors.New("remote interest")
	ErrRemoteIntent    = errors.New("remote intent")
)
