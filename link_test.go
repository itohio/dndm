package dndm

import (
	"context"
	"testing"
	"time"

	"github.com/itohio/dndm/testutil"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/proto"
)

func Test_NewLink(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*10)
	defer cancel()
	intent := &MockIntentInternal{}
	interest := &MockInterestInternal{}
	intent.On("Link", (chan<- proto.Message)(nil))
	intent.On("Ctx").Return(ctx)
	interest.On("Ctx").Return(ctx)

	link := NewLink(ctx, intent, interest)
	onClose := testutil.NewFunc(ctx, t, "close link")
	link.AddOnClose(onClose.F)
	assert.NotNil(t, link)
	assert.Equal(t, intent, link.intent)
	assert.Equal(t, interest, link.interest)
	onClose.NotCalled()
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

	link := NewLink(ctx, intent, interest)
	link.Link()

	link.Notify()

	onClose := testutil.NewFunc(ctx, t, "close link")
	link.AddOnClose(onClose.F)

	link.Close()

	onClose.WaitCalled()
	assert.True(t, testutil.CtxRecv(ctx, link.Ctx().Done()))
	time.Sleep(time.Millisecond)
	// Expect Link to have been called with msgC and nil
	intent.AssertExpectations(t)
	interest.AssertExpectations(t)
}
