// Code generated by mockery v2.42.3. DO NOT EDIT.

package network

import (
	context "context"

	dndm "github.com/itohio/dndm"
	core "github.com/itohio/dndm/types/core"

	mock "github.com/stretchr/testify/mock"

	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
)

// MockConn is an autogenerated mock type for the Conn type
type MockConn struct {
	mock.Mock
}

type MockConn_Expecter struct {
	mock *mock.Mock
}

func (_m *MockConn) EXPECT() *MockConn_Expecter {
	return &MockConn_Expecter{mock: &_m.Mock}
}

// AddRoute provides a mock function with given fields: _a0
func (_m *MockConn) AddRoute(_a0 ...dndm.Route) {
	_va := make([]interface{}, len(_a0))
	for _i := range _a0 {
		_va[_i] = _a0[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockConn_AddRoute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'AddRoute'
type MockConn_AddRoute_Call struct {
	*mock.Call
}

// AddRoute is a helper method to define mock.On call
//   - _a0 ...dndm.Route
func (_e *MockConn_Expecter) AddRoute(_a0 ...interface{}) *MockConn_AddRoute_Call {
	return &MockConn_AddRoute_Call{Call: _e.mock.On("AddRoute",
		append([]interface{}{}, _a0...)...)}
}

func (_c *MockConn_AddRoute_Call) Run(run func(_a0 ...dndm.Route)) *MockConn_AddRoute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]dndm.Route, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(dndm.Route)
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *MockConn_AddRoute_Call) Return() *MockConn_AddRoute_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockConn_AddRoute_Call) RunAndReturn(run func(...dndm.Route)) *MockConn_AddRoute_Call {
	_c.Call.Return(run)
	return _c
}

// Close provides a mock function with given fields:
func (_m *MockConn) Close() error {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Close")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockConn_Close_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Close'
type MockConn_Close_Call struct {
	*mock.Call
}

// Close is a helper method to define mock.On call
func (_e *MockConn_Expecter) Close() *MockConn_Close_Call {
	return &MockConn_Close_Call{Call: _e.mock.On("Close")}
}

func (_c *MockConn_Close_Call) Run(run func()) *MockConn_Close_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockConn_Close_Call) Return(_a0 error) *MockConn_Close_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConn_Close_Call) RunAndReturn(run func() error) *MockConn_Close_Call {
	_c.Call.Return(run)
	return _c
}

// DelRoute provides a mock function with given fields: _a0
func (_m *MockConn) DelRoute(_a0 ...dndm.Route) {
	_va := make([]interface{}, len(_a0))
	for _i := range _a0 {
		_va[_i] = _a0[_i]
	}
	var _ca []interface{}
	_ca = append(_ca, _va...)
	_m.Called(_ca...)
}

// MockConn_DelRoute_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'DelRoute'
type MockConn_DelRoute_Call struct {
	*mock.Call
}

// DelRoute is a helper method to define mock.On call
//   - _a0 ...dndm.Route
func (_e *MockConn_Expecter) DelRoute(_a0 ...interface{}) *MockConn_DelRoute_Call {
	return &MockConn_DelRoute_Call{Call: _e.mock.On("DelRoute",
		append([]interface{}{}, _a0...)...)}
}

func (_c *MockConn_DelRoute_Call) Run(run func(_a0 ...dndm.Route)) *MockConn_DelRoute_Call {
	_c.Call.Run(func(args mock.Arguments) {
		variadicArgs := make([]dndm.Route, len(args)-0)
		for i, a := range args[0:] {
			if a != nil {
				variadicArgs[i] = a.(dndm.Route)
			}
		}
		run(variadicArgs...)
	})
	return _c
}

func (_c *MockConn_DelRoute_Call) Return() *MockConn_DelRoute_Call {
	_c.Call.Return()
	return _c
}

func (_c *MockConn_DelRoute_Call) RunAndReturn(run func(...dndm.Route)) *MockConn_DelRoute_Call {
	_c.Call.Return(run)
	return _c
}

// Local provides a mock function with given fields:
func (_m *MockConn) Local() Peer {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Local")
	}

	var r0 Peer
	if rf, ok := ret.Get(0).(func() Peer); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(Peer)
	}

	return r0
}

// MockConn_Local_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Local'
type MockConn_Local_Call struct {
	*mock.Call
}

// Local is a helper method to define mock.On call
func (_e *MockConn_Expecter) Local() *MockConn_Local_Call {
	return &MockConn_Local_Call{Call: _e.mock.On("Local")}
}

func (_c *MockConn_Local_Call) Run(run func()) *MockConn_Local_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockConn_Local_Call) Return(_a0 Peer) *MockConn_Local_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConn_Local_Call) RunAndReturn(run func() Peer) *MockConn_Local_Call {
	_c.Call.Return(run)
	return _c
}

// OnClose provides a mock function with given fields: _a0
func (_m *MockConn) OnClose(_a0 func()) Conn {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for OnClose")
	}

	var r0 Conn
	if rf, ok := ret.Get(0).(func(func()) Conn); ok {
		r0 = rf(_a0)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(Conn)
		}
	}

	return r0
}

// MockConn_OnClose_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'OnClose'
type MockConn_OnClose_Call struct {
	*mock.Call
}

// OnClose is a helper method to define mock.On call
//   - _a0 func()
func (_e *MockConn_Expecter) OnClose(_a0 interface{}) *MockConn_OnClose_Call {
	return &MockConn_OnClose_Call{Call: _e.mock.On("OnClose", _a0)}
}

func (_c *MockConn_OnClose_Call) Run(run func(_a0 func())) *MockConn_OnClose_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(func()))
	})
	return _c
}

func (_c *MockConn_OnClose_Call) Return(_a0 Conn) *MockConn_OnClose_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConn_OnClose_Call) RunAndReturn(run func(func()) Conn) *MockConn_OnClose_Call {
	_c.Call.Return(run)
	return _c
}

// Read provides a mock function with given fields: ctx
func (_m *MockConn) Read(ctx context.Context) (*core.Header, protoreflect.ProtoMessage, error) {
	ret := _m.Called(ctx)

	if len(ret) == 0 {
		panic("no return value specified for Read")
	}

	var r0 *core.Header
	var r1 protoreflect.ProtoMessage
	var r2 error
	if rf, ok := ret.Get(0).(func(context.Context) (*core.Header, protoreflect.ProtoMessage, error)); ok {
		return rf(ctx)
	}
	if rf, ok := ret.Get(0).(func(context.Context) *core.Header); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*core.Header)
		}
	}

	if rf, ok := ret.Get(1).(func(context.Context) protoreflect.ProtoMessage); ok {
		r1 = rf(ctx)
	} else {
		if ret.Get(1) != nil {
			r1 = ret.Get(1).(protoreflect.ProtoMessage)
		}
	}

	if rf, ok := ret.Get(2).(func(context.Context) error); ok {
		r2 = rf(ctx)
	} else {
		r2 = ret.Error(2)
	}

	return r0, r1, r2
}

// MockConn_Read_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Read'
type MockConn_Read_Call struct {
	*mock.Call
}

// Read is a helper method to define mock.On call
//   - ctx context.Context
func (_e *MockConn_Expecter) Read(ctx interface{}) *MockConn_Read_Call {
	return &MockConn_Read_Call{Call: _e.mock.On("Read", ctx)}
}

func (_c *MockConn_Read_Call) Run(run func(ctx context.Context)) *MockConn_Read_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context))
	})
	return _c
}

func (_c *MockConn_Read_Call) Return(_a0 *core.Header, _a1 protoreflect.ProtoMessage, _a2 error) *MockConn_Read_Call {
	_c.Call.Return(_a0, _a1, _a2)
	return _c
}

func (_c *MockConn_Read_Call) RunAndReturn(run func(context.Context) (*core.Header, protoreflect.ProtoMessage, error)) *MockConn_Read_Call {
	_c.Call.Return(run)
	return _c
}

// Remote provides a mock function with given fields:
func (_m *MockConn) Remote() Peer {
	ret := _m.Called()

	if len(ret) == 0 {
		panic("no return value specified for Remote")
	}

	var r0 Peer
	if rf, ok := ret.Get(0).(func() Peer); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(Peer)
	}

	return r0
}

// MockConn_Remote_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Remote'
type MockConn_Remote_Call struct {
	*mock.Call
}

// Remote is a helper method to define mock.On call
func (_e *MockConn_Expecter) Remote() *MockConn_Remote_Call {
	return &MockConn_Remote_Call{Call: _e.mock.On("Remote")}
}

func (_c *MockConn_Remote_Call) Run(run func()) *MockConn_Remote_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run()
	})
	return _c
}

func (_c *MockConn_Remote_Call) Return(_a0 Peer) *MockConn_Remote_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConn_Remote_Call) RunAndReturn(run func() Peer) *MockConn_Remote_Call {
	_c.Call.Return(run)
	return _c
}

// UpdateRemotePeer provides a mock function with given fields: _a0
func (_m *MockConn) UpdateRemotePeer(_a0 Peer) error {
	ret := _m.Called(_a0)

	if len(ret) == 0 {
		panic("no return value specified for UpdateRemotePeer")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(Peer) error); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockConn_UpdateRemotePeer_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'UpdateRemotePeer'
type MockConn_UpdateRemotePeer_Call struct {
	*mock.Call
}

// UpdateRemotePeer is a helper method to define mock.On call
//   - _a0 Peer
func (_e *MockConn_Expecter) UpdateRemotePeer(_a0 interface{}) *MockConn_UpdateRemotePeer_Call {
	return &MockConn_UpdateRemotePeer_Call{Call: _e.mock.On("UpdateRemotePeer", _a0)}
}

func (_c *MockConn_UpdateRemotePeer_Call) Run(run func(_a0 Peer)) *MockConn_UpdateRemotePeer_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(Peer))
	})
	return _c
}

func (_c *MockConn_UpdateRemotePeer_Call) Return(_a0 error) *MockConn_UpdateRemotePeer_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConn_UpdateRemotePeer_Call) RunAndReturn(run func(Peer) error) *MockConn_UpdateRemotePeer_Call {
	_c.Call.Return(run)
	return _c
}

// Write provides a mock function with given fields: ctx, route, msg
func (_m *MockConn) Write(ctx context.Context, route dndm.Route, msg protoreflect.ProtoMessage) error {
	ret := _m.Called(ctx, route, msg)

	if len(ret) == 0 {
		panic("no return value specified for Write")
	}

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, dndm.Route, protoreflect.ProtoMessage) error); ok {
		r0 = rf(ctx, route, msg)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// MockConn_Write_Call is a *mock.Call that shadows Run/Return methods with type explicit version for method 'Write'
type MockConn_Write_Call struct {
	*mock.Call
}

// Write is a helper method to define mock.On call
//   - ctx context.Context
//   - route dndm.Route
//   - msg protoreflect.ProtoMessage
func (_e *MockConn_Expecter) Write(ctx interface{}, route interface{}, msg interface{}) *MockConn_Write_Call {
	return &MockConn_Write_Call{Call: _e.mock.On("Write", ctx, route, msg)}
}

func (_c *MockConn_Write_Call) Run(run func(ctx context.Context, route dndm.Route, msg protoreflect.ProtoMessage)) *MockConn_Write_Call {
	_c.Call.Run(func(args mock.Arguments) {
		run(args[0].(context.Context), args[1].(dndm.Route), args[2].(protoreflect.ProtoMessage))
	})
	return _c
}

func (_c *MockConn_Write_Call) Return(_a0 error) *MockConn_Write_Call {
	_c.Call.Return(_a0)
	return _c
}

func (_c *MockConn_Write_Call) RunAndReturn(run func(context.Context, dndm.Route, protoreflect.ProtoMessage) error) *MockConn_Write_Call {
	_c.Call.Return(run)
	return _c
}

// NewMockConn creates a new instance of MockConn. It also registers a testing interface on the mock and a cleanup function to assert the mocks expectations.
// The first argument is typically a *testing.T value.
func NewMockConn(t interface {
	mock.TestingT
	Cleanup(func())
}) *MockConn {
	mock := &MockConn{}
	mock.Mock.Test(t)

	t.Cleanup(func() { mock.AssertExpectations(t) })

	return mock
}