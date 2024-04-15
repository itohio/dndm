package dndm

import (
	"fmt"
	"reflect"

	"google.golang.org/protobuf/proto"
)

// Route describes named and typed data route that essentially consists of the path and data type.
type Route struct {
	route   string
	path    string
	msgType reflect.Type
}

func (r Route) Equal(route Route) bool {
	return r.route == route.route
}

func (r Route) ID() string {
	return r.route
}

func (r Route) String() string {
	return r.route
}

func (r Route) Bytes() []byte {
	return []byte(r.route)
}

func NewRoute(path string, msg proto.Message) (Route, error) {
	return Route{
		route:   fmt.Sprintf("%s@%s", path, proto.MessageName(msg)),
		path:    path,
		msgType: reflect.TypeOf(msg),
	}, nil
}

func RouteFromString(route string) (Route, error) {
	return Route{
		route: route,
	}, nil
}

func (r Route) Route() Route {
	return r
}

func (r Route) Path() string {
	return r.path
}

func (r Route) Type() reflect.Type {
	return r.msgType
}
