// Package direct provides a concrete implementation of the dndm.Endpoint interface,
// facilitating direct communications by managing links between intents and interests.
// It integrates robust lifecycle management and ensures proper resource handling
// through its BaseEndpoint and custom Linker logic.
package direct

import (
	"context"
	"log/slog"

	"github.com/itohio/dndm"
)

var _ dndm.Endpoint = (*Endpoint)(nil)

type Endpoint struct {
	dndm.BaseEndpoint
	linker *dndm.Linker
}

// New creates and returns a new instance of Endpoint with specified buffer size.
// The size parameter affects internal buffering and link management capacities.
func New(size int) *Endpoint {
	return &Endpoint{
		BaseEndpoint: dndm.NewEndpointBase("direct", size),
	}
}

// OnClose registers a callback function that will be invoked when the endpoint is closed.
// This method allows for cleanup activities or notifications to be scheduled upon closure.
func (t *Endpoint) OnClose(f func()) dndm.Endpoint {
	t.AddOnClose(f)
	return t
}

// Close terminates the Endpoint and cleans up resources, particularly closing the internal Linker
// if it has been initialized. It ensures all associated resources are properly released.
func (t *Endpoint) Close() error {
	t.BaseEndpoint.Close()
	if t.linker == nil {
		return nil
	}
	return t.linker.Close()
}

// Init initializes the Endpoint with necessary context, logging, and callbacks for intents and interests.
// This setup is crucial for the Endpoint to function properly within a networked environment, ensuring
// that it can handle incoming and outgoing data flows as expected.
func (t *Endpoint) Init(ctx context.Context, logger *slog.Logger, addIntent dndm.IntentCallback, addInterest dndm.InterestCallback) error {
	if err := t.BaseEndpoint.Init(ctx, logger, addIntent, addInterest); err != nil {
		return err
	}

	t.linker = dndm.NewLinker(
		ctx, logger, t.Size,
		func(intent dndm.Intent) error {
			return addIntent(intent, t)
		},
		func(interest dndm.Interest) error {
			return addInterest(interest, t)
		},
		nil,
	)
	return nil
}

// Publish creates and registers a new intent based on the provided route. This method
// leverages the internal Linker to manage the intent lifecycle and connectivity.
func (t *Endpoint) Publish(route dndm.Route, opt ...dndm.PubOpt) (dndm.Intent, error) {
	intent, err := t.linker.AddIntent(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("intent registered", "intent", route)
	return intent, nil
}

// Subscribe creates and registers a new interest based on the provided route. Similar to Publish,
// this method uses the internal Linker to manage the interest and ensure it is properly integrated
// within the networked environment.
func (t *Endpoint) Subscribe(route dndm.Route, opt ...dndm.SubOpt) (dndm.Interest, error) {
	interest, err := t.linker.AddInterest(route)
	if err != nil {
		return nil, err
	}
	t.Log.Info("registered", "interest", route)
	return interest, nil
}
