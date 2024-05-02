package dndm

import (
	"reflect"
	"testing"

	testtypes "github.com/itohio/dndm/types/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestNewRoute(t *testing.T) {
	msg := &testtypes.Foo{}
	route, err := NewRoute("path", msg)
	assert.NoError(t, err)
	expectedRoute := PlainRoute{route: "types.Foo@path", path: "path", msgType: reflect.TypeOf(msg)}
	assert.Equal(t, expectedRoute, route)

	assert.Equal(t, "types.Foo@path", route.ID())
	assert.Equal(t, "types.Foo@path", route.String())
	assert.Equal(t, "path", route.Path())
	assert.Equal(t, reflect.TypeOf(msg), route.Type())

	_, err = NewRoute("", msg)
	assert.Error(t, err)
	_, err = NewRoute("a@b", msg)
	assert.Error(t, err)
	_, err = NewRoute("a#b", msg)
	assert.Error(t, err)
}

func TestNewHashedRoute(t *testing.T) {
	msg := &testtypes.Foo{}
	route, err := NewHashedRoute("example", "example.path", msg)
	assert.NoError(t, err)
	expectedRoute := HashedRoute{route: "m1F+jLCmA+RQL1TkllbKW4cFj8I", prefix: "example", msgType: reflect.TypeOf(msg)}
	assert.Equal(t, expectedRoute, route)

	assert.Equal(t, "m1F+jLCmA+RQL1TkllbKW4cFj8I", route.ID())
	assert.Equal(t, "example#m1F+jLCmA+RQL1TkllbKW4cFj8I", route.String())
	assert.Equal(t, "example", route.Path())
	assert.Equal(t, reflect.TypeOf(msg), route.Type())

	_, err = NewHashedRoute("", "example.path", msg)
	assert.Error(t, err)
	_, err = NewHashedRoute("example", "exampl", msg)
	assert.Error(t, err)
	_, err = NewHashedRoute("example", "", msg)
	assert.Error(t, err)
	_, err = NewHashedRoute("@", "example.path", msg)
	assert.Error(t, err)
	_, err = NewHashedRoute("#", "example.path", msg)
	assert.Error(t, err)

	_, err = NewHashedRoute("ex", "example@path", msg)
	assert.NoError(t, err)
	_, err = NewHashedRoute("ex", "example#path", msg)
	assert.NoError(t, err)
}

func TestRouteFromString(t *testing.T) {
	route, err := RouteFromString("MessageType@path")
	assert.NoError(t, err)

	assert.Equal(t, "MessageType@path", route.ID())
	assert.Equal(t, "MessageType@path", route.String())
	assert.Equal(t, "path", route.Path())
	assert.Equal(t, nil, route.Type())

	_, err = RouteFromString("@path")
	assert.Error(t, err)
	_, err = RouteFromString("foo@")
	assert.Error(t, err)
	_, err = RouteFromString("")
	assert.Error(t, err)
}

func TestRouteFromStringHashed(t *testing.T) {
	route, err := RouteFromString("example#m1F+jLCmA+RQL1TkllbKW4cFj8I")
	require.NoError(t, err)

	assert.Equal(t, "m1F+jLCmA+RQL1TkllbKW4cFj8I", route.ID())
	assert.Equal(t, "example#m1F+jLCmA+RQL1TkllbKW4cFj8I", route.String())
	assert.Equal(t, "example", route.Path())
	assert.Equal(t, nil, route.Type())
}

func TestRouteFromStringHashedNoPrefix(t *testing.T) {
	route, err := RouteFromString("#m1F+jLCmA+RQL1TkllbKW4cFj8I")
	require.NoError(t, err)

	assert.Equal(t, "m1F+jLCmA+RQL1TkllbKW4cFj8I", route.ID())
	assert.Equal(t, "#m1F+jLCmA+RQL1TkllbKW4cFj8I", route.String())
	assert.Equal(t, "", route.Path())
	assert.Equal(t, nil, route.Type())
}
