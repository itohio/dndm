# DNBus Request/Reply Example

This example demonstrates using the `dnbus` wrappers for request/reply patterns.

## Features Demonstrated

- **Caller**: Type-safe request client with single reply via `Call()`
- **Service**: Type-safe request handler with goroutine-based processing
- **Multiple replies**: Shows how to send/receive multiple replies per request

## Running

```bash
go run examples/dnbus_request/main.go
```

## What It Does

1. Creates a service that handles requests
2. Creates a caller that sends requests and waits for replies
3. Demonstrates both single reply and multiple reply patterns

## Key Differences from Raw API

### Before (Raw API):
```go
// Manual correlation, type assertions, interest waiting
var req *Request
var resp *Response
requestIntent, _ := router.Publish("requests", req)
responseInterest, _ := router.Subscribe("responses", resp)
// ... manual correlation, nonce handling, etc.
```

### After (DNBus):
```go
// Service
service, _ := dnbus.NewService[*Request, *Response](ctx, router, "requests", "responses")
service.Handle(ctx, func(ctx context.Context, req *Request, reply func(resp *Response) error) error {
    return reply(&Response{...})
})

// Caller
caller, _ := dnbus.NewCaller[*Request, *Response](ctx, router, "requests", "responses")
resp, err := caller.Call(ctx, &Request{...})
```

