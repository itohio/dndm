# Makefile for embedded example

.PHONY: all build clean test proto docker-build docker-run

# Build both components
all: build

# Generate protobuf Go code
proto:
	cd proto && buf generate

# Build Go binaries
build:
	go build -o bin/embedded ./embedded
	go build -o bin/host ./host

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf types/embedded.pb.go

# Run tests (if any)
test:
	go test ./...

# Build Docker image
docker-build:
	docker build -t dndm-embedded .

# Run with docker-compose
docker-run:
	docker-compose up --build

# Stop docker-compose
docker-stop:
	docker-compose down

# Clean everything
docker-clean: docker-stop
	docker system prune -f
	docker volume prune -f
