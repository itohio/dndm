package dndm

import (
	"crypto/sha1"
	"encoding/base64"
	"reflect"
	"strings"

	"github.com/itohio/dndm/errors"
	"google.golang.org/protobuf/proto"
)

var (
	_ Route = (*PlainRoute)(nil)
	_ Route = (*HashedRoute)(nil)
)

// RemoteRoute describes a named and typed data route. It encapsulates a routing mechanism
// by combining the path of the route with the protobuf message type, facilitating
// the identification and handling of different data types across a distributed system.
// Remote Route is a subset of Route that does not implement Type() method.
type RemoteRoute interface {
	// ID returns the unique identifier of the route, which combines the path
	// and the name of the protobuf message type.
	ID() string
	// Path returns the path component of the route.
	Path() string

	// Equal checks if two Route instances represent the same route.
	// It returns true if both routes have the same route identifier.
	Equal(Route) bool
	// String returns the route as a string, which is the unique identifier
	// combining the path with the protobuf message type name.
	String() string
}

// Route describes a named and typed data route. It encapsulates a routing mechanism
// by combining the path of the route with the protobuf message type, facilitating
// the identification and handling of different data types across a distributed system.
//
//   - Routes can be plain and hashed. Plain routes contain the path and type description.
//   - Remote routes may not contain the type so that Type method returns nil or the interface is not implemented at all.
//   - Hashed routes are (some-hash) of the path and type. This way it is possible to hide types of the messages.
//     Hashing in this case allows for Object-capability security model where only those who know exact path and
//     exact type can send and decode received message.
type Route interface {
	RemoteRoute
	// Type returns the reflect.Type of the proto.Message associated with the route,
	// allowing type introspection and dynamic handling of message types.
	//
	// Type() returns nil for routes received from remote endpoints.
	Type() reflect.Type
}

type PlainRoute struct {
	route   string
	path    string
	msgType reflect.Type
}

type HashedRoute struct {
	route   string
	prefix  string
	msgType reflect.Type
}

// EmptyRoute creates an empty route that is useful in tests and also in remote endpoint.
func EmptyRoute() PlainRoute {
	return PlainRoute{}
}

func routeString(path string, msg proto.Message) string {
	return string(proto.MessageName(msg)) + "@" + path
}

// NewRoute creates a new Plain text Route instance given a path and a proto.Message.
// The route's identifier is formed by concatenating the provided path with the
// name of the message's type, separated by an "@" symbol.
//
// Path must not contain `@` nor `#` symbols.
func NewRoute(path string, msg proto.Message) (PlainRoute, error) {
	if strings.ContainsAny(path, "@#") {
		return PlainRoute{}, errors.ErrInvalidRoute
	}
	return PlainRoute{
		route:   routeString(path, msg),
		path:    path,
		msgType: reflect.TypeOf(msg),
	}, nil
}

// NewHashedRoute creates a new Hashed Route instance given a path and a proto.Message.
// The route's identifier is formed by concatenating the provided prefix with the
// hashed route, separated by an "#" symbol.
//
// Route is represented by the hash, while the prefix is used for routing Remote Interests and Intents.
func NewHashedRoute(prefix, path string, msg proto.Message) (HashedRoute, error) {
	if strings.ContainsAny(prefix, "@#") {
		return HashedRoute{}, errors.ErrInvalidRoute
	}
	if !strings.HasPrefix(path, prefix) {
		return HashedRoute{}, errors.ErrInvalidRoutePrefix
	}
	route := routeString(path, msg)
	hash := sha1.Sum([]byte(route))
	return HashedRoute{
		route:   base64.RawStdEncoding.EncodeToString(hash[:]),
		prefix:  prefix,
		msgType: reflect.TypeOf(msg),
	}, nil
}

// RouteFromString creates a Route from a string representation,
// assuming the string is a valid route identifier.
//
//   - Plain Route `Type@Path`
//   - Hashed Ruote `prefix#[Base64 Hash]`
func RouteFromString(route string) (Route, error) {
	if strings.Contains(route, "@") {
		return plainRouteFromString(route)
	}
	if strings.Contains(route, "#") {
		return hashedRouteFromString(route)
	}
	return PlainRoute{}, errors.ErrInvalidRoute
}

func plainRouteFromString(route string) (PlainRoute, error) {
	parts := strings.Split(route, "@")
	if len(parts) != 2 {
		return PlainRoute{}, errors.ErrInvalidRoute
	}
	return PlainRoute{
		route: route,
		path:  parts[1],
	}, nil
}

func hashedRouteFromString(route string) (HashedRoute, error) {
	parts := strings.Split(route, "#")
	if len(parts) != 2 {
		return HashedRoute{}, errors.ErrInvalidRoute
	}
	return HashedRoute{
		route:  parts[1],
		prefix: parts[0],
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

func (r PlainRoute) Path() string {
	return r.path
}

func (r PlainRoute) Type() reflect.Type {
	return r.msgType
}

func (r HashedRoute) Equal(route Route) bool {
	return r.ID() == route.ID()
}

func (r HashedRoute) ID() string {
	return r.route
}

func (r HashedRoute) String() string {
	return r.prefix + "#" + r.route
}

func (r HashedRoute) Path() string {
	return r.prefix
}

func (r HashedRoute) Type() reflect.Type {
	return r.msgType
}
