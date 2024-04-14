package main

import (
	"context"
	"io"
)

// ReadWriter stores pointers to a Reader and a Writer.
// It implements io.ReadWriter.
type ReadWriter struct {
	r io.Reader
	w io.Writer
}

func (rw ReadWriter) Read(p []byte) (n int, err error) {
	return rw.r.Read(p)
}
func (rw ReadWriter) Write(p []byte) (n int, err error) {
	return rw.w.Write(p)
}

// NewReadWriter allocates a new ReadWriter that dispatches to r and w.
func NewReadWriter(r io.Reader, w io.Writer) *ReadWriter {
	return &ReadWriter{r, w}
}

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

func (b bridge) A() io.ReadWriter {
	return NewReadWriter(b.rA, b.wB)
}

func (b bridge) B() io.ReadWriter {
	return NewReadWriter(b.rB, b.wA)
}
