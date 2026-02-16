package ch04

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// --- STEP 3 ---
// ## Dynamically Allocating the Buffer Size
/*
- You can read data of variable length from a network connection, provided that both the sender and receiver have agreed on a protocol for doing so.
	- The type-length-value (TLV) encoding scheme is a good option.
  		- TLV encoding uses a fixed number of bytes to represent the type of data,
 		- a fixed number of bytes to represent the value size,
 		- and a variable number of bytes to represent the value itself.
	- Our implementation uses a 5-byte header: 1 byte for the type and 4 bytes for the length.
	- The TLV encoding scheme allows you to send a type as a series of bytes to a remote node
	- and constitute the same type on the remote node from the series of bytes.
	- Listing 4-4 defines the types that our TLV encoding protocol will accept.
*/
// Listing 4-4: The message struct implements a simple protocol
/*
- What does TLV mean?
	- TLV stands for:
		- T = Type (message type)
		- Example: 1 means “Text”, 2 means “Image”, 3 means “Ping”, …
	- L = Length (message length)
		- This means how many bytes of data are going to come after it.
	- V = Value (data itself)
		- The actual payload.
- Why does a TLV help?
	- Because TCP is not message-oriented; it only sends a stream of bytes.
		- A TLV lets the receiver know exactly:
			- How many bytes to read first for the header
			- From the header, figure out what the message type is
			- From the length, figure out how many bytes are the payload
			- Then create and read a buffer of exactly the same size
- What does "5-byte header" mean in this book?
	- It says their protocol is:
		- 1 byte for Type
   		- 4 bytes for Length
   		- Total 5 bytes header
   		- So the message structure is:
			- [Type:1 byte][Length:4 bytes][Value:Length bytes]
*/
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

// Listing 4-5: Creating the Binary type
type Binary []byte

func (m Binary) Bytes() []byte  { return m }
func (m Binary) String() string { return string(m) }

func (m Binary) WriteTo(w io.Writer) (int64, error) {
	err := binary.Write(w, binary.BigEndian, BinaryType) // 1-byte type
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	err = binary.Write(w, binary.BigEndian, uint32(len(m))) // 4-byte size
	if err != nil {
		return n, err
	}
	n += 4

	o, err := w.Write(m) // payload

	return n + int64(o), err
}
