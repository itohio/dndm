package dndm

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func Test_NewLink(t *testing.T) {
	ctx := context.Background()
	intent := &MockIntentInternal{}
	interest := &MockInterestInternal{}
	closer := func() error { return nil }
	intent.On("Ctx").Return(ctx)
	interest.On("Ctx").Return(ctx)

	link := NewLink(ctx, intent, interest, closer)
	assert.NotNil(t, link)
	assert.Equal(t, intent, link.intent)
	assert.Equal(t, interest, link.interest)
}

func TestLink_Link(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	intent := &MockIntentInternal{}
	interest := &MockInterestInternal{}
	ch := make(chan proto.Message)
	intent.On("Link", (chan<- proto.Message)(ch))
	intent.On("Link", (chan<- proto.Message)(nil))
	intent.On("Ctx").Return(ctx)
	intent.On("Notify").Twice()
	interest.On("MsgC").Return((chan<- proto.Message)(ch))
	interest.On("Ctx").Return(ctx)

	closer := func() error { return nil }

	link := NewLink(ctx, intent, interest, closer)
	link.Link()

	link.Notify()

	link.Unlink()
	_, ok := <-link.done
	assert.False(t, ok)

	fmt.Println(intent.Calls)

	// Expect Link to have been called with msgC and nil
	intent.AssertExpectations(t)
	interest.AssertExpectations(t)
}
