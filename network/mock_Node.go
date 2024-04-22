// Code generated by mockery v2.42.3. DO NOT EDIT.

package network

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"
)

// MockNode is an autogenerated mock type for the Node type
type MockNode struct {
	mock.Mock
}

type MockNode_Expecter struct {
	mock *mock.Mock
}

func (_m *MockNode) EXPECT() *MockNode_Expecter {
	return &MockNode_Expecter{mock: &_m.Mock}
}

// Dial provides a mock function with given fields: ctx, peer, o
func (_m *MockNode) Dial(ctx context.Context, peer Peer, o ...DialOpt) (io.ReadWriteCloser, error) {
	_va := make([]interface{}, len(o))
	for _i := range o {
		_va[_i] = o[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, peer)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Dial")
	}

	var r0 io.ReadWriteCloser
	var r1 error
	if rf, ok := ret.Get(0).(func(context.Context, Peer, ...DialOpt) (io.ReadWriteCloser, error)); ok {
		return rf(ctx, peer, o...)
	}
	if rf, ok := ret.Get(0).(func(context.Context, Peer, ...DialOpt) io.ReadWriteCloser); ok {
		r0 = rf(ctx, peer, o...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(io.ReadWriteCloser)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context, Peer, ...DialOpt) error); ok {
		r1 = rf(ctx, peer, o...)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// MockNode_Dial_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Dial'
type MockNode_Dial_Call struct {
	*mock.Call
}

// Dial is a helper method to define mock.On call
//   - ctx context.Context
//   - peer Peer
//   - o ...DialOpt
func (_e *MockNode_Expecter) Dial(ctx interface{}, peer interface{}, o ...interface{}) *MockNode_Dial_Call {
	return &MockNode_Dial_Call{Call: _e.mock.On("Dial",
		append([]interface{}{ctx, peer}, o...)...)}
}

func (_c *MockNode_Dial_Call) Run(run func(ctx context.Context, peer Peer, o ...DialOpt)) *MockNode_Dial_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]DialOpt, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(DialOpt)
			}
		}
		run(args[0].(context.Context), args[1].(Peer), variadicArgs...)
	})
	return _c
}

func (_c *MockNode_Dial_Call) Return(_a0 io.ReadWriteCloser, _a1 error) *MockNode_Dial_Call {
	_c.Call.Return(_a0, _a1)
	return _c
}

func (_c *MockNode_Dial_Call) RunAndReturn(run func(context.Context, Peer, ...DialOpt) (io.ReadWriteCloser, error)) *MockNode_Dial_Call {
	_c.Call.Return(run)
	return _c
}

// Scheme provides a mock function with given fields:
func (_m *MockNode) Scheme() string {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Scheme")
	}

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// MockNode_Scheme_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Scheme'
type MockNode_Scheme_Call struct {
	*mock.Call
}

// Scheme is a helper method to define mock.On call
func (_e *MockNode_Expecter) Scheme() *MockNode_Scheme_Call {
	return &MockNode_Scheme_Call{Call: _e.mock.On("Scheme")}
}

func (_c *MockNode_Scheme_Call) Run(run func()) *MockNode_Scheme_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockNode_Scheme_Call) Return(_a0 string) *MockNode_Scheme_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockNode_Scheme_Call) RunAndReturn(run func() string) *MockNode_Scheme_Call {
	_c.Call.Return(run)
	return _c
}

// Serve provides a mock function with given fields: ctx, onConnect, o
func (_m *MockNode) Serve(ctx context.Context, onConnect func(Peer, io.ReadWriteCloser) error, o ...SrvOpt) error {
	_va := make([]interface{}, len(o))
	for _i := range o {
		_va[_i] = o[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, ctx, onConnect)
	_ca = append(_ca, _va...)
	ret := _m.Called(_ca...)

	if len(ret) == 0 {
		panic("no return value specified for Serve")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, func(Peer, io.ReadWriteCloser) error, ...SrvOpt) error); ok {
		r0 = rf(ctx, onConnect, o...)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockNode_Serve_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Serve'
type MockNode_Serve_Call struct {
	*mock.Call
}

// Serve is a helper method to define mock.On call
//   - ctx context.Context
//   - onConnect func(Peer , io.ReadWriteCloser) error
//   - o ...SrvOpt
func (_e *MockNode_Expecter) Serve(ctx interface{}, onConnect interface{}, o ...interface{}) *MockNode_Serve_Call {
	return &MockNode_Serve_Call{Call: _e.mock.On("Serve",
		append([]interface{}{ctx, onConnect}, o...)...)}
}

func (_c *MockNode_Serve_Call) Run(run func(ctx context.Context, onConnect func(Peer, io.ReadWriteCloser) error, o ...SrvOpt)) *MockNode_Serve_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]SrvOpt, len(args)-2)
		for i, a := range args[2:] {
			if a != nil {
				variadicArgs[i] = a.(SrvOpt)
			}
		}
		run(args[0].(context.Context), args[1].(func(Peer, io.ReadWriteCloser) error), variadicArgs...)
	})
	return _c
}

func (_c *MockNode_Serve_Call) Return(_a0 error) *MockNode_Serve_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockNode_Serve_Call) RunAndReturn(run func(context.Context, func(Peer, io.ReadWriteCloser) error, ...SrvOpt) error) *MockNode_Serve_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockNode creates a new instance of MockNode. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockNode(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockNode {
	mock := &MockNode{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}
