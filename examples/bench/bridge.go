package main

import (
	"bufio"
	"context"
	"io"
)

type bridge struct {
	// pipe A
	rA io.Reader
	wA io.Writer
	// pipe B
	rB io.Reader
	wB io.Writer
}

func makeBridge(ctx context.Context) *bridge {
	rA, wA := io.Pipe()
	rB, wB := io.Pipe()

	return &bridge{
		rA: rA,
		wA: wA,
		rB: rB,
		wB: wB,
	}
}

func (b bridge) A() *bufio.ReadWriter {
	return bufio.NewReadWriter(bufio.NewReader(b.rA), bufio.NewWriter(b.wB))
}

func (b bridge) B() *bufio.ReadWriter {
	return bufio.NewReadWriter(bufio.NewReader(b.rB), bufio.NewWriter(b.wA))
}
