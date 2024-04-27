package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type FuncMock struct {
	ctx    context.Context
	t      *testing.T
	name   string
	called chan struct{}
}

func NewFunc(ctx context.Context, t *testing.T, name string) FuncMock {
	return FuncMock{
		ctx:    ctx,
		t:      t,
		name:   name,
		called: make(chan struct{}),
	}
}

func (c FuncMock) F() {
	c.t.Log(c.name)
	close(c.called)
}
func (c FuncMock) FE(e error) func() error {
	return func() error {
		c.F()
		return e
	}
}

func (c FuncMock) WaitCalled() {
	assert.True(c.t, CtxRecv(c.ctx, c.called))
}
func (c FuncMock) WaitNotCalled() {
	assert.True(c.t, CtxRecv(c.ctx, c.called))
}
func (c FuncMock) Called() {
	assert.True(c.t, IsClosed(c.called))
}
func (c FuncMock) NotCalled() {
	assert.False(c.t, IsClosed(c.called))
}

func IsClosed[T any](c <-chan T) bool {
	select {
	case _, ok := <-c:
		return !ok
	default:
		return false
	}
}

func CtxRecv[T any](ctx context.Context, c <-chan T) bool {
	select {
	case <-ctx.Done():
		return false
	case <-c:
		return true
	}
}
