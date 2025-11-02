# DNBus Direct Example

This example demonstrates using the `dnbus` wrappers for producer/consumer patterns with the direct endpoint.

## Features Demonstrated

- **Producer**: Type-safe producer that automatically waits for interest
- **Consumer**: Type-safe consumer with automatic type conversion
- **Multiple message types**: Shows how different message types (Foo, Bar) can use the same route

## Running

```bash
go run examples/dnbus_direct/main.go
```

## What It Does

1. Creates a producer that sends `Foo` messages
2. Creates consumers that receive `Foo` and `Bar` messages
3. Creates a producer that sends `Bar` messages
4. All messages use the same route (`example.foobar`) but different types

## Key Differences from Raw API

### Before (Raw API):
```go
var t *types.Foo
intent, err := node.Publish("example.foobar", t)
defer intent.Close()
select {
case <-intent.Interest():
    intent.Send(ctx, msg)
}
```

### After (DNBus):
```go
producer, err := dnbus.NewProducer[*types.Foo](ctx, router, "example.foobar")
defer producer.Close()
producer.Send(ctx, msg) // Automatically waits for interest
```

