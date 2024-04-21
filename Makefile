# Define the paths to the source code and build artifacts
BUILDDIR=./build

# Define the name of the binary and Docker image
# Get a list of directories in ./examples
TARGETS := $(notdir $(shell find ./examples -mindepth 1 -maxdepth 1 -type d))
BUILD_TARGETS := $(addprefix build-,$(TARGETS))

# Find directories with Dockerfiles
DOCKER_DIRS := $(wildcard ./examples/*/Dockerfile)
DOCKER_TARGETS := $(notdir $(dir $(DOCKER_DIRS)))
BUILD_DOCKER_TARGETS := $(addprefix docker-,$(DOCKER_TARGETS))

# Define the build flags for go build
BUILD_FLAGS=-ldflags="-s -w"

.PHONY: all
all: test build build-docker

.PHONY: build
build: $(BUILD_TARGETS)

.PHONY: proto
proto:
	cd proto &&	buf generate 

.PHONY: mocks
mocks:
	mockgen -source=intent.go -package testutil -destination=testutil/intent_mocks.go
	mockgen -source=interest.go -package testutil -destination=testutil/interest_mocks.go
	mockgen -source=endpoint.go -package testutil -destination=testutil/endpoint_mocks.go
	mockgen -source=network/network.go -package testutil -destination=testutil/network_mocks.go

.PHONY: gen
gen:
	# Run go generate to generate any required files
	go generate ./...

$(BUILD_TARGETS): 
	# Build the production binary
	go build $(BUILD_FLAGS) -o $(BUILDDIR)/$(@:build-%=%) ./cmd/$(@:build-%=%)


.PHONY: vulncheck

.PHONY: vulncheck
vulncheck: $(BUILDDIR)/
	GOBIN=$(BUILDDIR) go install golang.org/x/vuln/cmd/govulncheck@latest
	$(BUILDDIR)/govulncheck ./...

.PHONY: cover
cover:
	go test -coverprofile=.coverage .
	go tool cover -func=.coverage

.PHONY: test
test:
	go test ./...

.PHONY: bench
bench:
	go test -bench=./...

ifeq (,$(N))
  N := 100
endif

.PHONY: run-bench
run-bench:
	for i in {1..$(N)}; do go run  ./examples/bench $(ARGS) 2>&1; done

.PHONY: clean
clean:
	# Remove the build artifacts
	rm -rf $(BUILDDIR)


.PHONY: help
help:
	@echo "Makefile for building, testing, and Dockerizing a Go application"
	@echo ""
	@echo "Usage:"
	@echo "  make gen             Generate necessary files"
	@echo "  make proto           Compile proto files"
	@echo "  make mocks           Update interface mocks"
	@echo "  make all           	Run tests, build binaries, and build docker images"
	@echo "  make build         	Build all binaries"
	@echo "  make build-{target}  Build the production binary that is in ./cmd/{target}"
	@echo "  make vulncheck       Test packages for known vulnerabilities"
	@echo "  make cover           Run test coverage analysis"
	@echo "  make test            Run all tests"
	@echo "  make bench           Run all benchmark tests"
	@echo "  make run-bench       Run end to end benchmark test"
	@echo "  make run-bench       Run example benchmark, N argument for run count, ARGS as arguments to executable"
	@echo "  make clean           Remove build artifacts"
	@echo "  make help            Display this help message"

.DEFAULT_TARGET: help
