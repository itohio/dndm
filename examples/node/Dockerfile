#
# Default values
#
ARG BINARY_NAME=node

#
# 1. Build Container
#
FROM golang:1.21 AS build

# Copy necessary files for dependency resolution
WORKDIR /home/src
COPY go.mod go.sum ./

# Download dependencies
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the entire source code
COPY . .

# Build the binary
RUN make build-node

#
# 2. Runtime Container
#
FROM alpine as publisher

ENV TZ=UTC

# Copy the binary from the build container
WORKDIR /home/app
COPY --from=build /home/src/dist/${BINARY_NAME} .

CMD ["./${BINARY_NAME}"]
