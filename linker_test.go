package dndm

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/itohio/dndm/testutil"
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
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()

	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	linker := NewLinker(ctx, slog.Default(), 10,
		func(intent Intent) error { return addIntent.FE(nil)() },
		func(interest Interest) error { return addInterest.FE(nil)() },
		nil,
	)
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultIntent, err := linker.AddIntent(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultIntent)
	assert.Contains(t, linker.intents, route.ID())
	addIntent.WaitCalled()

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

	addIntent := testutil.NewFunc(ctx, t, "add intent")
	addInterest := testutil.NewFunc(ctx, t, "add interest")
	linker := NewLinker(ctx, slog.Default(), 10,
		func(intent Intent) error { return addIntent.FE(nil)() },
		func(interest Interest) error { return addInterest.FE(nil)() },
		nil,
	)
	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultInterest, err := linker.AddInterest(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultInterest)
	assert.Contains(t, linker.interests, route.ID())
	onClose := testutil.NewFunc(ctx, t, "close interest")
	resultInterest.OnClose(onClose.F)

	addInterest.WaitCalled()

	foundInterest, ok := linker.Interest(route)
	assert.True(t, ok)
	assert.Equal(t, resultInterest, foundInterest)

	time.Sleep(time.Millisecond)
	addIntent.NotCalled()
	onClose.NotCalled()

	err = linker.RemoveInterest(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.interests, route.ID())

	time.Sleep(time.Millisecond)
	onClose.NotCalled()

	linker.Close()

	time.Sleep(time.Millisecond)
	onClose.WaitCalled()

	<-linker.Ctx().Done()

	assert.Equal(t, context.Canceled, linker.ctx.Err())
}

func TestLinker_AddIntentInterestSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	addIntent := testutil.NewFunc(ctx, t, "addIntent")
	addInterest := testutil.NewFunc(ctx, t, "addInterest")
	beforeLink := testutil.NewFunc(ctx, t, "before Link")
	linker := NewLinker(ctx, slog.Default(), 10,
		func(intent Intent) error { return addIntent.FE(nil)() },
		func(interest Interest) error { return addInterest.FE(nil)() },
		func(i1 Intent, i2 Interest) error { return beforeLink.FE(nil)() },
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultInterest, err := linker.AddInterest(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultInterest)
	assert.Contains(t, linker.interests, route.ID())

	time.Sleep(time.Millisecond)
	addInterest.Called()

	onDelInterest := testutil.NewFunc(ctx, t, "close interest")
	resultInterest.OnClose(onDelInterest.F)

	resultIntent, err := linker.AddIntent(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultIntent)
	assert.Contains(t, linker.intents, route.ID())

	addIntent.WaitCalled()
	beforeLink.WaitCalled()
	assert.True(t, testutil.CtxRecv(ctx, resultIntent.Interest()))

	msg := &testtypes.Foo{Text: "text"}
	err = resultIntent.Send(ctx, msg)
	assert.NoError(t, err)

	v, err := recvChan(resultInterest.C(), time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, msg, v)

	err = linker.RemoveInterest(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.interests, route.ID())

	linker.Close()

	addIntent.WaitCalled()
	addInterest.WaitCalled()
	onDelInterest.WaitCalled()

	<-linker.ctx.Done()

	assert.Equal(t, context.Canceled, linker.ctx.Err())
}

func TestLinker_AddInterestIntentSend(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*100)
	defer cancel()

	addIntent := testutil.NewFunc(ctx, t, "addIntent")
	addInterest := testutil.NewFunc(ctx, t, "addInterest")
	beforeLink := testutil.NewFunc(ctx, t, "before Link")
	linker := NewLinker(ctx, slog.Default(), 10,
		func(intent Intent) error { return addIntent.FE(nil)() },
		func(interest Interest) error { return addInterest.FE(nil)() },
		func(i1 Intent, i2 Interest) error { return beforeLink.FE(nil)() },
	)

	route, err := NewRoute("path", &testtypes.Foo{})
	require.NoError(t, err)

	resultIntent, err := linker.AddIntent(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultIntent)
	assert.Contains(t, linker.intents, route.ID())

	addIntent.WaitCalled()

	resultInterest, err := linker.AddInterest(route)
	assert.NoError(t, err)
	assert.NotNil(t, resultInterest)
	assert.Contains(t, linker.interests, route.ID())

	addInterest.WaitCalled()
	beforeLink.WaitCalled()

	assert.True(t, testutil.CtxRecv(ctx, resultIntent.Interest()))

	msg := &testtypes.Foo{Text: "text"}
	err = resultIntent.Send(ctx, msg)
	assert.NoError(t, err)

	v, err := recvChan(resultInterest.C(), time.Millisecond)
	assert.NoError(t, err)
	assert.Equal(t, msg, v)

	removeInterest := testutil.NewFunc(ctx, t, "remove Interest")
	resultInterest.OnClose(removeInterest.F)

	err = linker.RemoveInterest(route)
	assert.NoError(t, err)
	assert.NotContains(t, linker.interests, route.ID())

	time.Sleep(time.Millisecond)
	removeInterest.NotCalled()

	linker.Close()

	<-linker.ctx.Done()

	assert.Equal(t, context.Canceled, linker.ctx.Err())
}
