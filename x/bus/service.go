package bus

import (
	"context"

	"github.com/itohio/dndm"
	"google.golang.org/protobuf/proto"
)

// Service handles incoming requests and sends replies, reusing Consumer for requests
// and Producer for responses. Each request is handled in a separate goroutine.
type Service[Req proto.Message, Resp proto.Message] struct {
	requestConsumer  *Consumer[Req]
	responseProducer *Producer[Resp]
	router           *dndm.Router
	requestPath      string
	responsePath     string
}

// NewService creates a request handler service that receives requests on requestPath
// and sends replies on responsePath.
func NewService[Req proto.Message, Resp proto.Message](
	ctx context.Context,
	router *dndm.Router,
	requestPath string,
	responsePath string,
) (*Service[Req, Resp], error) {
	requestConsumer, err := NewConsumer[Req](ctx, router, requestPath)
	if err != nil {
		return nil, err
	}

	responseProducer, err := NewProducer[Resp](ctx, router, responsePath)
	if err != nil {
		requestConsumer.Close()
		return nil, err
	}

	return &Service[Req, Resp]{
		requestConsumer:  requestConsumer,
		responseProducer: responseProducer,
		router:           router,
		requestPath:      requestPath,
		responsePath:     responsePath,
	}, nil
}

// Handle handles requests with a callback (goroutine-based).
// The callback is called in a goroutine for each request and can send one or multiple replies.
func (s *Service[Req, Resp]) Handle(ctx context.Context, handler func(ctx context.Context, req Req, reply func(resp Resp) error) error) error {
	// Wait for interest on response producer once
	if err := s.responseProducer.WaitForInterest(ctx); err != nil {
		return err
	}

	for {
		req, err := s.requestConsumer.Receive(ctx)
		if err != nil {
			return err
		}

		go func(request Req) {
			replyFunc := func(resp Resp) error {
				return s.responseProducer.SendDirect(ctx, resp)
			}

			if err := handler(ctx, request, replyFunc); err != nil {
				// TODO: optional error reporting hook
				_ = err
			}
		}(req)
	}
}

// Receive receives requests and calls the handler for each request (manual handling).
func (s *Service[Req, Resp]) Receive(ctx context.Context) (Req, error) {
	return s.requestConsumer.Receive(ctx)
}

// Reply sends a reply for a request (manual handling).
// Can be called multiple times to send multiple replies.
func (s *Service[Req, Resp]) Reply(ctx context.Context, resp Resp) error {
	// SendDirect waits for interest if needed and sends without closing intent
	return s.responseProducer.SendDirect(ctx, resp)
}

// Close closes the service and releases associated resources.
func (s *Service[Req, Resp]) Close() error {
	var err1, err2 error
	if s.requestConsumer != nil {
		err1 = s.requestConsumer.Close()
	}
	if s.responseProducer != nil {
		err2 = s.responseProducer.Close()
	}
	if err1 != nil {
		return err1
	}
	return err2
}
