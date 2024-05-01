package dndm

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

// Route describes a named and typed data route. It encapsulates a routing mechanism
// by combining the path of the route with the protobuf message type, facilitating
// the identification and handling of different data types across a distributed system.
//
//   - Routes can be plain and hashed. Plain routes contain the path and type description.
//   - Remote routes may not contain the type so that Type method returns nil.
//   - Hashed routes are (some-hash) of the path and type. This way it is possible to hide types of the messages.
//     Hashing in this case allows for Object-capability security model where only those who know exact path and
//     exact type can send and decode received message.
type Route interface {
	// ID returns the unique identifier of the route, which combines the path
	// and the name of the protobuf message type.
	ID() string
	// Path returns the path component of the route.
	Path() string
	// Type returns the reflect.Type of the proto.Message associated with the route,
	// allowing type introspection and dynamic handling of message types.
	//
	// Type() returns nil for routes received from remote endpoints.
	Type() reflect.Type

	// Equal checks if two Route instances represent the same route.
	// It returns true if both routes have the same route identifier.
	Equal(Route) bool
	// String returns the route as a string, which is the unique identifier
	// combining the path with the protobuf message type name.
	String() string
	// Bytes returns the byte representation of the route's unique identifier.
	Bytes() []byte
}

type PlainRoute struct {
	route   string
	path    string
	msgType reflect.Type
}

// EmptyRoute creates an empty route that is useful in tests and also in remote endpoint.
func EmptyRoute() PlainRoute {
	return PlainRoute{}
}

// NewRoute creates a new Route instance given a path and a proto.Message.
// The route's identifier is formed by concatenating the provided path with the
// name of the message's type, separated by an "@" symbol.
func NewRoute(path string, msg proto.Message) (PlainRoute, error) {
	return PlainRoute{
		route:   fmt.Sprintf("%s@%s", proto.MessageName(msg), path),
		path:    path,
		msgType: reflect.TypeOf(msg),
	}, nil
}

// RouteFromString creates a Route from a string representation,
// assuming the string is a valid route identifier.
func RouteFromString(route string) (PlainRoute, error) {
	return PlainRoute{
		route: route,
	}, nil
}

func (r PlainRoute) Equal(route Route) bool {
	return r.ID() == route.ID()
}

func (r PlainRoute) ID() string {
	return r.route
}

func (r PlainRoute) String() string {
	return r.route
}

func (r PlainRoute) Bytes() []byte {
	return []byte(r.route)
}

func (r PlainRoute) Path() string {
	return r.path
}

func (r PlainRoute) Type() reflect.Type {
	return r.msgType
}
