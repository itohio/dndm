# Embedded Example

This example demonstrates a complete embedded system scenario using DNDM x/bus primitives. It consists of two components that communicate over serial or TCP:

1. **Embedded Component** (`embedded/`): Runs on RP2040/TinyGo, publishes telemetry and provides control services
2. **Host Component** (`host/`): Runs on host system, consumes telemetry and calls services

## Architecture

### Embedded Component (RP2040)
- **Path**: `example.rp2040`
- **Counter Intent**: Publishes incremental counter values every 100ms
- **Control Interest**: Consumes control messages with single `N` field (float32)
- **Service**: Provides request/reply service accepting `int` and `float32`, returning computed result

### Host Component
- **Path**: `example.host`
- **Counter Consumer**: Consumes embedded counter, logs every 10th value, detects gaps
- **Control Producer**: Sends control commands with `N` value from keyboard input
- **Service Caller**: Calls embedded service and verifies responses

## Building and Running

### Local Development (Serial)

1. **Generate protobuf code** (if needed):
   ```bash
   make proto
   ```

2. **Build components**:
   ```bash
   make build
   ```

3. **Connect serial devices** (RP2040 on `/dev/ttyACM0`)

4. **Run embedded component** (on RP2040 or simulator):
   ```bash
   ./bin/embedded
   ```

5. **Run host component** (on host):
   ```bash
   ./bin/host
   ```

### Docker Testing (TCP)

For testing without hardware:

1. **Build and run with docker-compose**:
   ```bash
   make docker-run
   ```

2. **Interact with host**:
   ```bash
   docker-compose exec host sh
   ./host/host
   ```

## Usage

### Host Commands

Once the host is running, use these commands:

- `control <N>` - Send control value (e.g., `control 3.14`)
- `request <int> <float>` - Call service (e.g., `request 42 2.71`)
- `quit` - Exit

### Example Session

```
Enter commands:
  control <N> - send control value
  request <int> <float> - call service
  quit - exit
control 1.5
request 10 2.5
request 5 3.0
quit
```

## Transport Configuration

Set the `TRANSPORT` environment variable:

- `TRANSPORT=serial` - Use serial communication (default)
- `TRANSPORT=tcp` - Use TCP for testing

## Protocol Buffers

The example uses these protobuf messages:

```protobuf
message Counter {
  int32 value = 1;
}

message Control {
  float n = 1;
}

message Request {
  int32 int_val = 1;
  float float_val = 2;
}

message Response {
  float result = 1;
  int32 counter = 2;
  float control_n = 3;
}
```

## Files

- `proto/types/embedded.proto` - Protocol buffer definitions
- `proto/buf.yaml` - Buf linting configuration
- `proto/buf.gen.yaml` - Buf code generation configuration
- `types/embedded.pb.go` - Generated Go types
- `embedded/main.go` - Embedded device implementation
- `host/main.go` - Host implementation
- `PLAN.md` - Implementation plan and analysis
- `docker-compose.yml` - Docker testing setup
- `Dockerfile` - Docker build configuration
- `Makefile` - Build and test automation
