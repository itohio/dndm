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
			route1:   PlainRoute{route: "MessageType@path"},
			route2:   PlainRoute{route: "MessageType@path"},
			expected: true,
		},
		{
			name:     "Different routes",
			route1:   PlainRoute{route: "MessageType@path1"},
			route2:   PlainRoute{route: "MessageType@path2"},
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
	route := PlainRoute{route: "MessageType@path"}
	assert.Equal(t, "MessageType@path", route.ID())
}

func TestRoute_String(t *testing.T) {
	route := PlainRoute{route: "MessageType@path"}
	assert.Equal(t, "MessageType@path", route.String())
}

func TestRoute_Bytes(t *testing.T) {
	route := PlainRoute{route: "MessageType@path"}
	assert.Equal(t, []byte("MessageType@path"), route.Bytes())
}

func TestNewRoute(t *testing.T) {
	msg := &testtypes.Foo{}
	route, err := NewRoute("path", msg)
	assert.NoError(t, err)
	expectedRoute := PlainRoute{route: "types.Foo@path", path: "path", msgType: reflect.TypeOf(msg)}
	assert.Equal(t, expectedRoute, route)
}

func TestRouteFromString(t *testing.T) {
	route, err := RouteFromString("MessageType@path")
	assert.NoError(t, err)
	expectedRoute := PlainRoute{route: "MessageType@path"}
	assert.Equal(t, expectedRoute, route)
}

func TestRoute_Path(t *testing.T) {
	route := PlainRoute{route: "MessageType@path", path: "path"}
	assert.Equal(t, "path", route.Path())
}

func TestRoute_Type(t *testing.T) {
	msg := &testtypes.Bar{}
	route := PlainRoute{route: "YourMessageType@path", msgType: reflect.TypeOf(msg)}
	assert.Equal(t, reflect.TypeOf(msg), route.Type())
}
