package main

import (
	"errors"
	"fmt"
	"io"
)

// Listing 4-4: The message struct implements a simple protocol
const (
	BinaryType uint8 = iota + 1 // (1)
	StringType
	MaxPayloadSize uint32 = 10 << 20 // 10 MB
)

var ErrMaxPayloadSize = errors.New("maximum payload size exceeded")

type Payload interface {
	fmt.Stringer
	io.ReaderFrom
	io.WriterTo
	Bytes() []byte
}
