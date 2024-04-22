package dndm

import (
	"reflect"
	"testing"

	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
)

func TestRoute_Equal(t *testing.T) {
	tests := []struct {
		name     string
		route1   Route
		route2   Route
		expected bool
	}{
		{
			name:     "Equal routes",
			route1:   Route{route: "path@MessageType"},
			route2:   Route{route: "path@MessageType"},
			expected: true,
		},
		{
			name:     "Different routes",
			route1:   Route{route: "path1@MessageType"},
			route2:   Route{route: "path2@MessageType"},
			expected: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := test.route1.Equal(test.route2)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestRoute_ID(t *testing.T) {
	route := Route{route: "path@MessageType"}
	assert.Equal(t, "path@MessageType", route.ID())
}

func TestRoute_String(t *testing.T) {
	route := Route{route: "path@MessageType"}
	assert.Equal(t, "path@MessageType", route.String())
}

func TestRoute_Bytes(t *testing.T) {
	route := Route{route: "path@MessageType"}
	assert.Equal(t, []byte("path@MessageType"), route.Bytes())
}

func TestNewRoute(t *testing.T) {
	msg := &testtypes.Foo{}
	route, err := NewRoute("path", msg)
	assert.NoError(t, err)
	expectedRoute := Route{route: "path@types.Foo", path: "path", msgType: reflect.TypeOf(msg)}
	assert.Equal(t, expectedRoute, route)
}

func TestRouteFromString(t *testing.T) {
	route, err := RouteFromString("path@MessageType")
	assert.NoError(t, err)
	expectedRoute := Route{route: "path@MessageType"}
	assert.Equal(t, expectedRoute, route)
}

func TestRoute_Route(t *testing.T) {
	route := Route{route: "path@MessageType"}
	assert.Equal(t, route, route.Route())
}

func TestRoute_Path(t *testing.T) {
	route := Route{route: "path@MessageType", path: "path"}
	assert.Equal(t, "path", route.Path())
}

func TestRoute_Type(t *testing.T) {
	msg := &testtypes.Bar{}
	route := Route{route: "path@YourMessageType", msgType: reflect.TypeOf(msg)}
	assert.Equal(t, reflect.TypeOf(msg), route.Type())
}
