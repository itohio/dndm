// Code generated by mockery v2.42.3. DO NOT EDIT.

package network

import (
	context "context"
	io "io"

	mock "github.com/stretchr/testify/mock"
)

// MockServer is an autogenerated mock type for the Server type
type MockServer struct {
	mock.Mock
}

type MockServer_Expecter struct {
	mock *mock.Mock
}

func (_m *MockServer) EXPECT() *MockServer_Expecter {
	return &MockServer_Expecter{mock: &_m.Mock}
}

// Serve provides a mock function with given fields: ctx, onConnect, o
func (_m *MockServer) Serve(ctx context.Context, onConnect func(Peer, io.ReadWriteCloser) error, o ...SrvOpt) error {
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

// MockServer_Serve_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Serve'
type MockServer_Serve_Call struct {
	*mock.Call
}

// Serve is a helper method to define mock.On call
//   - ctx context.Context
//   - onConnect func(Peer , io.ReadWriteCloser) error
//   - o ...SrvOpt
func (_e *MockServer_Expecter) Serve(ctx interface{}, onConnect interface{}, o ...interface{}) *MockServer_Serve_Call {
	return &MockServer_Serve_Call{Call: _e.mock.On("Serve",
		append([]interface{}{ctx, onConnect}, o...)...)}
}

func (_c *MockServer_Serve_Call) Run(run func(ctx context.Context, onConnect func(Peer, io.ReadWriteCloser) error, o ...SrvOpt)) *MockServer_Serve_Call {
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

func (_c *MockServer_Serve_Call) Return(_a0 error) *MockServer_Serve_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockServer_Serve_Call) RunAndReturn(run func(context.Context, func(Peer, io.ReadWriteCloser) error, ...SrvOpt) error) *MockServer_Serve_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockServer creates a new instance of MockServer. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockServer(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockServer {
	mock := &MockServer{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}