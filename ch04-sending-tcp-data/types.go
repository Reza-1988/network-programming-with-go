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

// Listing 4-4: The message struct implements a simple protocol
/*
- You start by creating constants to represent each type you will define.
	- In this example, you will create a BinaryType (1) and a StringType (2).
	- After digesting the implementation details of each type, you should be able to create types that fit your needs.
	- For security purposes that we’ll discuss in just a moment, you must define a maximum payload size (3).
- You also define an interface named Payload (4) that describes the methods each type must implement.
	- Each type must have the following methods: Bytes, String, ReadFrom, and WriteTo.
		- The io.ReaderFrom and io.WriterTo interfaces allow your types to read from readers and write to writers, respectively.
		- You have some flexibility in this regard.
		- You could just as easily make the Payload implement the `encoding.BinaryMarshaler` interface to marshal itself to a byte slice and
		- the `encoding.BinaryUnmarshaler` interface to unmarshal itself from a byte slice.
		- But the byte slice is one level removed from the network connection, so you’ll keep the Payload interface as is.
		- Besides, you’ll use the binary encoding interfaces in the next chapter.
- You now have the foundation built to create TLV-based types.
*/
/*
1) What are these consts for?
	- (1) and (2) Defining message types (Type)
		- In TLV, Type is usually a small number (here 1 byte = uint8).
  			- BinaryType means the message type is “binary” (e.g. file, image, any raw data)
   			- StringType means the message type is “text”
		- iota is an automatic counter in Go.
			- iota starts at 0.
			- Since we have + 1, then:
				- BinaryType becomes 1
				- StringType becomes 2
	- Result:
		- BinaryType = 1
		- StringType = 2
	- This is exactly the same as T in TLV:
		- That is, when you send the message, you write Type=1 or Type=2 in the header so that the other party knows how to interpret the payload.
	- (3) Why do we have MaxPayloadSize?
		- This means we limit the maximum payload size to 10MB.
			- 10 << 20 means: 10 times 2^20
				- That is 10 * 1,048,576 ≈ 10,485,760 bytes ≈ 10MB
		- Why is this important?
			- For security and problem prevention:
				- If the other party (or attacker) says: "Message length = 4GB"
				- And you try to create a 4GB buffer → RAM will burst and the application will crash (DoS)
			- Then this value is a "safety ceiling".
	- << means Left Shift. Very simple:
		- x << n means you shift the number x, n bits to the left
		- The result is usually:
			- x * (2^n)
*/
//
const (
	BinaryType     uint8  = iota + 1 // (1)
	StringType                       // (2)
	MaxPayloadSize uint32 = 10 << 20 // 10 MB (3)
)

// 2) What is this error for?
// 	- If the payload length is greater than MaxPayloadSize , we return this error. In simple terms:
// 		- The message you are sending/receiving is too large, I will not accept it.

var ErrMaxPayloadSize = errors.New("maximum payload size exceeded")

/*
3) What does Payload interface mean?
	- Here we define a “contract” for each payload type (such as String or Binary).
	- That is, anything that wants to be a payload must have these capabilities:
	- 3.1) What does fmt.Stringer mean?
		- This interface means it must have the following method:
			- `String() string`
			- So the payload should be able to convert itself to a string for printing/logging/debugging.
			- For example, if the payload is text, String() will return that text.
	- 3.2) What does io.ReaderFrom mean?
		- This means that the payload must be able to read itself from a Reader.
		- Method: `ReadFrom(r io.Reader) (n int64, err error)` That is:
			- You give the payload an `r` (can be `conn`)
			- The payload reads itself and stores it inside itself
			- `n` says how many bytes were read
		- Note: net.Conn itself is an io.Reader, so it can read the payload directly from the network.
	- 3.3) What does `io.WriterTo` mean?
		- This means that the payload must be able to write itself to a Writer.
		- Method: `WriteTo(w io.Writer) (n int64, err error)` Meaning:
			- The payload writes itself to `w`
			- `w` can be the same as `conn` (since `net.Conn` is also a writer)
	- 3.4) What does `Bytes() []byte` mean?
		- They added this method themselves (it is not part of io).
		- That is, the payload should be able to return its own byte version:
			- For cases where you want to have raw bytes
			- For example, for hashing, storing, or testing
*/

type Payload interface { //(4)
	fmt.Stringer
	io.ReaderFrom
	io.WriterTo
	Bytes() []byte
}

// Listing 4-5: Creating the Binary type
type Binary []byte // (1)

func (m Binary) Bytes() []byte  { return m }         // (2)
func (m Binary) String() string { return string(m) } // (3)

func (m Binary) WriteTo(w io.Writer) (int64, error) { // (4)
	err := binary.Write(w, binary.BigEndian, BinaryType) // 1-byte type (5)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	err = binary.Write(w, binary.BigEndian, uint32(len(m))) // 4-byte size (6)
	if err != nil {
		return n, err
	}
	n += 4

	o, err := w.Write(m) // payload (7)

	return n + int64(o), err
}

// Listing 4-6: Completing the Binary type’s implementation ( types go)

func (m *Binary) ReadFrom(r io.Reader) (int64, error) {
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ) // 1-byte type
	if err != nil {
		return 0, err
	}
	var n int64 = 1
	if typ != BinaryType {
		return n, errors.New("invalid Binary")
	}

	var size uint32
	err = binary.Read(r, binary.BigEndian, &size) // 4-byte size
	if err != nil {
		return n, err
	}
	n += 4
	if size > MaxPayloadSize {
		return n, ErrMaxPayloadSize
	}

	*m = make([]byte, size)
	o, err := r.Read(*m) // payload

	return n + int64(o), err
}
