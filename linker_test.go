package dndm

import (
	"context"
	"log/slog"
	"testing"
	"time"

	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_NewLinker(t *testing.T) {
	ctx := context.Background()
	linker := NewLinker(ctx, slog.Default(), 10, nil, nil, nil)
	assert.NotNil(t, linker)

	assert.NotNil(t, linker.beforeLink)

	err := linker.Close()
	assert.NoError(t, err)

	beforeLink := func(i1 Intent, i2 Interest) error { return nil }
	linker = NewLinker(ctx, slog.Default(), 10, nil, nil, beforeLink)
	assert.NotNil(t, linker)
	//assert.True(t, beforeLink, linker.beforeLink)

	err = linker.Close()
	assert.NoError(t, err)
}

func TestLinker_AddRemoveIntent(t *testing.T) {
	ctx := context.Background()
	linker := NewLinker(ctx, slog.Default(), 10, nil, nil, nil)
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultIntent, err := linker.AddIntent(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultIntent)
	assert.Contains(t, linker.intents, route.ID())
	foundIntent, ok := linker.Intent(route)
	assert.True(t, ok)
	assert.Equal(t, resultIntent, foundIntent)

	err = linker.RemoveIntent(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.intents, route.ID())
}

func TestLinker_AddRemoveInterest(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	addCalled := make(chan struct{})
	delCalled := make(chan struct{})
	linker := NewLinker(ctx, slog.Default(), 10,
		func(interest Interest) error {
			close(addCalled)
			return nil
		},
		func(interest Interest) error {
			close(delCalled)
			return nil
		},
		nil,
	)
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultInterest, err := linker.AddInterest(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultInterest)
	assert.Contains(t, linker.interests, route.ID())
	foundInterest, ok := linker.Interest(route)
	assert.True(t, ok)
	assert.Equal(t, resultInterest, foundInterest)

	_, ok = linker.Intent(route)
	assert.False(t, ok)

	assert.True(t, ctxRecv(ctx, addCalled))

	err = linker.RemoveInterest(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.interests, route.ID())

	assert.True(t, ctxRecv(ctx, delCalled))

	linker.Close()

	<-linker.ctx.Done()

	assert.Equal(t, context.DeadlineExceeded, linker.ctx.Err())
}

func TestLinker_AddIntentInterestSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	addCalled := make(chan struct{})
	delCalled := make(chan struct{})
	beforeLinkCalled := make(chan struct{})
	linker := NewLinker(ctx, slog.Default(), 10,
		func(interest Interest) error {
			close(addCalled)
			return nil
		},
		func(interest Interest) error {
			close(delCalled)
			return nil
		},
		func(i1 Intent, i2 Interest) error {
			close(beforeLinkCalled)
			return nil
		},
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultInterest, err := linker.AddInterest(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultInterest)
	assert.Contains(t, linker.interests, route.ID())

	assert.True(t, ctxRecv(ctx, addCalled))

	resultIntent, err := linker.AddIntent(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultIntent)
	assert.Contains(t, linker.intents, route.ID())

	assert.True(t, ctxRecv(ctx, beforeLinkCalled))
	assert.True(t, ctxRecv(ctx, resultIntent.Interest()))

	msg := &testtypes.Foo{Text: "text"}
	err = resultIntent.Send(ctx, msg)
	assert.NoError(t, err)

	v, err := recvChan(resultInterest.C(), time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, msg, v)

	err = linker.RemoveInterest(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.interests, route.ID())

	assert.True(t, ctxRecv(ctx, delCalled))

	linker.Close()

	<-linker.ctx.Done()

	assert.Equal(t, context.DeadlineExceeded, linker.ctx.Err())
}

func TestLinker_AddInterestIntentSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	addCalled := make(chan struct{})
	delCalled := make(chan struct{})
	beforeLinkCalled := make(chan struct{})
	linker := NewLinker(ctx, slog.Default(), 10,
		func(interest Interest) error {
			close(addCalled)
			return nil
		},
		func(interest Interest) error {
			close(delCalled)
			return nil
		},
		func(i1 Intent, i2 Interest) error {
			close(beforeLinkCalled)
			return nil
		},
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultIntent, err := linker.AddIntent(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultIntent)
	assert.Contains(t, linker.intents, route.ID())

	resultInterest, err := linker.AddInterest(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultInterest)
	assert.Contains(t, linker.interests, route.ID())

	assert.True(t, ctxRecv(ctx, addCalled))

	assert.True(t, ctxRecv(ctx, beforeLinkCalled))
	assert.True(t, ctxRecv(ctx, resultIntent.Interest()))

	msg := &testtypes.Foo{Text: "text"}
	err = resultIntent.Send(ctx, msg)
	assert.NoError(t, err)

	v, err := recvChan(resultInterest.C(), time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, msg, v)

	err = linker.RemoveInterest(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.interests, route.ID())

	assert.True(t, ctxRecv(ctx, delCalled))

	linker.Close()

	<-linker.ctx.Done()

	assert.Equal(t, context.DeadlineExceeded, linker.ctx.Err())
}
