package testutil

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

type FuncMock struct {
	ctx    context.Context
	called chan struct{}
}

func NewFunc(ctx context.Context) FuncMock {
	return FuncMock{
		ctx:    ctx,
		called: make(chan struct{}),
	}
}

func (c FuncMock) F()        { close(c.called) }
func (c FuncMock) FE() error { close(c.called); return nil }

func (c FuncMock) WaitCalled(t *testing.T) {
	assert.True(t, CtxRecv(c.ctx, c.called))
}
func (c FuncMock) WaitNotCalled(t *testing.T) {
	assert.True(t, CtxRecv(c.ctx, c.called))
}
func (c FuncMock) Called(t *testing.T) {
	assert.True(t, IsClosed(c.called))
}
func (c FuncMock) NotCalled(t *testing.T) {
	assert.False(t, IsClosed(c.called))
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
