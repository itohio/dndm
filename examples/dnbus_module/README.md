# DNBus Module Example

This example demonstrates using the `dnbus` Module wrapper for multi-input/output processing modules.

## Features Demonstrated

- **Module**: Simplified management of multiple inputs and outputs
- **Input/Output**: Type-safe input and output wrappers
- **Processing pipeline**: Shows how to build processing modules

## Running

```bash
go run examples/dnbus_module/main.go
```

## What It Does

1. Creates a producer that feeds messages to a module
2. Creates a processing module with inputs and outputs
3. Processes messages and sends results to outputs

## Key Differences from Raw API

### Before (Raw API):
```go
// Manual creation of multiple intents/interests
var img *CameraImage
var sensor *SensorData
cameraInterest, _ := router.Subscribe("camera", img)
sensorInterest, _ := router.Subscribe("sensor", sensor)
// ... manual lifecycle management, error handling
```

### After (DNBus):
```go
module := dnbus.NewModule(ctx, router)
input1, _ := module.AddInput[*CameraImage]("camera")
input2, _ := module.AddInput[*SensorData]("sensor")
output, _ := module.AddOutput[*Result]("result")
module.Run(ctx, func(ctx context.Context) error {
    img, _ := input1.Receive(ctx)
    sensor, _ := input2.Receive(ctx)
    result := process(img, sensor)
    return output.Send(ctx, result)
})
```

