package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"time"

	"github.com/itohio/dndm"
	types "github.com/itohio/dndm/types/test"
	"google.golang.org/protobuf/proto"
)

func GenerateFoo(ctx context.Context, node *dndm.Router, name string) {
	if name == "" {
		return
	}

	var t *types.Foo
	intent, err := node.Publish(name, t)
	if err != nil {
		panic(err)
	}
	defer intent.Close()
	slog.Info("Publishing", "type", reflect.TypeOf(t), "name", name)
	i := 0
	for {
		select {
		case <-ctx.Done():
			slog.Info("generate foo ctx", "err", ctx.Err())
			return
		case route := <-intent.Interest():
			slog.Info("generate: interest", "route", route)

			for {
				time.Sleep(time.Millisecond * 500)
				select {
				case <-ctx.Done():
					return
				default:
				}

				text := fmt.Sprintf("Message: %d for %v as %s", i, reflect.TypeOf(t), name)
				c, cancel := context.WithTimeout(ctx, time.Second*10)
				err := intent.Send(c, &types.Foo{
					Text: text,
				})
				i++
				cancel()
				if err != nil {
					slog.Error("Failed sending Foo", "err", err)
					return
				}
			}
		}
	}
}

func Consume[T proto.Message](ctx context.Context, node *dndm.Router, name string) {
	if name == "" {
		return
	}

	var t T
	interest, err := node.Subscribe(name, t)
	if err != nil {
		panic(err)
	}
	defer interest.Close()
	slog.Info("Subscribing", "type", reflect.TypeOf(t), "name", name)
	for {
		select {
		case <-ctx.Done():
			slog.Info("consume ctx", "err", ctx.Err(), "type", reflect.TypeOf(t))
			return
		case msg := <-interest.C():
			m := msg.(T)

			buf, err := json.Marshal(m)
			if err != nil {
				panic(err)
			}
			slog.Info("Message", "route", interest.Route(), "msg", string(buf), "type", reflect.TypeOf(msg))
		}
	}
}
