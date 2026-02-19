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
1) What are these `consts` for?
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
/*
- The Binary type (1) is a byte slice; therefore, its Bytes method (2) simply returns itself.
- Its String method (3) casts itself as a string before returning.
- The WriteTo method accepts an `io.Writer` and returns the number of bytes written to the writer and an error interface (4).
	- The WriteTo method first writes the 1-byte type to the writer (5).
	- It then writes the 4-byte length of the Binary to the writer (6).
	- Finally, it writes the Binary value itself (7).
*/

// This code is creating a type of TLV message called Binary, which is "binary data" (i.e. a []byte),
// and then learning how to write itself to the network (or any Writer) as a TLV.
/*
0) What does the TLV look like here?
	- According to the book, we have a 5-byte header:
		- 1 byte: Type
		- 4 bytes: Length
		- Then: Value (the payload itself)
	- So the WriteTo output should look like this:
		- [Type (1 byte)] [Length (4 bytes)] [Value (len bytes)]
*/

// 1) Definition of Binary type
// 		- Binary is a new type
// 		- But it is actually the same as []byte (a slice of bytes)
// 		- So when you write: `m := Binary([]byte{1,2,3})`
// 			- You have a binary payload.

type Binary []byte // (1)

// 2) `Bytes()` method
// 		- Because Binary itself is `[]byte`
// 		- it doesn't need to do any special conversion
// 		- it just returns itself.

func (m Binary) Bytes() []byte { return m } // (2)

// 3) `String()` method
// 		- This method is only useful for printing/logging.
// 			- `string(m)` means "interpret these bytes as text".
// 				- If the text is really UTF-8, it will read correctly
// 				- If the data is really binary (image/file), the printout may look weird/unreadable.

func (m Binary) String() string { return string(m) } // (3)

// 4) `WriteTo()` method
//		- This method says:
//			- I (payload) want to write myself to a writer.
//			- Writer can be:
// 				- `net.Conn` or `bytes.Buffer` or file or anything that has `Write([]byte)`.
// 		- Output:
// 			- `int64` = number of bytes written
// 			- `error` = if there was a problem
//
// The Story of WriteTo (Step-by-Step)
// 		- Let's say `m` is our binary data (a []byte) that we want to send over the network.
// 		- What is the purpose of WriteTo?
// 			- It wants to write this to w:
// 				1.  Message type (Type) → 1 byte
// 				2. Message length (Length) → 4 bytes
// 				3. Data itself (Value / payload) → len(m) bytes
// 			- That is, like a postal envelope:
// 				- First you write on the envelope "What type of packet is this?"
// 				- Then you write "What is its size?"
// 				- Then you put the packet itself inside the envelope and send it

func (m Binary) WriteTo(w io.Writer) (int64, error) { // (4)

	// 4.1) Write Type (1 byte)
	// 		- What does it do here?
	// 			- `BinaryType` is a uint8 number (e.g. 1)
	// 			- `binary.Write` writes it to the writer as raw bytes.
	// 			- `binary.BigEndian` means that if it was multi-byte, the order is BigEndian (it doesn't matter for 1 byte, but it matters for length).
	// 		- Next:
	// 			- Since we wrote exactly 1 byte here, we set `n = 1`.

	err := binary.Write(w, binary.BigEndian, BinaryType) // 1-byte type (5)
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	// 4.2) Write Length (4 bytes)
	// 		- `len(m)` means the number of bytes of payload
	// 		- `uint32(len(m))` means we fit the length into 4 bytes
	// 		- `binary.Write` writes this length into 4 bytes.
	// 		- So far:
	// 			- 1 byte type
	// 			- 4 bytes length
	//			- Total 5 bytes header
	// 		- That's why we do n += 4 → n becomes 5.

	err = binary.Write(w, binary.BigEndian, uint32(len(m))) // 4-byte size (6)
	if err != nil {
		return n, err
	}
	n += 4

	// 4.3) Writing your own payload (Value)
	// 		- `w.Write(m)` sends the bytes itself.
	// 		- `o` is the number of bytes actually written.
	// 			- Usually equal to `len(m)`, but theoretically it can be less (some writers have partial writes).
	// 		- Finally:
	// 			- Total number of bytes written = n (header) + o (payload)
	// 			- And also returns err.

	o, err := w.Write(m) // payload (7)
	return n + int64(o), err
}

// Listing 4-6: Completing the Binary type’s implementation ( types go)
// 	- The `ReadFrom `method reads (1) 1 byte from the reader into the typ variable.
// 	- It next verifies (2) that the type is BinaryType before proceeding.
// 	- Then it reads (3) the next 4 bytes into the size variable, which sizes the new Binary byte slice (5).
//	- Finally, it populates the Binary byte slice (6).
//	- Notice that you enforce a maximum payload size 4.
//		- This is because the 4-byte integer you use to designate the payload size has a maximum value of 4,294,967,295, indicating a payload of over 4GB.
//		- With such a large payload size, it would be easy for a malicious actor to perform a denial-of-service attack that exhausts all the available random access memory (RAM) on your computer.
//		- Keeping the maximum payload size reasonable makes memory exhaustion attacks harder to execute.
//
// This `ReadFrom` is exactly the opposite of WriteTo: it reads a message TLV from the network/Reader and puts it into Binary.
//	- What is the general purpose of this function?
// 		- This message on the wire (network) looks like this:
// 			- Type → 1 byte
// 			- Length → 4 bytes
// 			- Value / Payload → Length bytes
//  	- `ReadFrom` wants to read these three pieces from `r` and finally fill `m` (the same Binary).
// 	- Why is receiver a pointer here? func (m *Binary)
//		- Because we are going to create and fill the value with `m` inside the function.
// 		- If there is no pointer, the changes are made to a copy and are not visible outside the function.
// 		- So *Binary means “change the actual Binary itself”.

func (m *Binary) ReadFrom(r io.Reader) (int64, error) {

	// 1) Read Type (1 byte)
	// 	- We create a `typ` variable for the message type
	// 	- `binary.Read` reads from r and puts it into typ
	// 	- Because typ is of type uint8 → exactly 1 byte is read from the stream.
	// 	- `var n int64 = 1` -> This means we have read 1 byte so far.
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ) // 1-byte type
	if err != nil {
		return 0, err
	}
	var n int64 = 1

	// 2) Check if the type is correct or not
	//	- Here it says:
	// 		- If the type you read was not `BinaryType`,
	// 		- That means this message is not of type Binary
	// 		- Then this function has no right to continue and will give an error.
	// 		- It's like the package says “String” but you said I only open “Binary” packages.

	if typ != BinaryType {
		return n, errors.New("invalid Binary")
	}

	// 3) Read Length (4 bytes)
	// 	- Now it reads the next 4 bytes
	// 	- and puts them into size
	// 	- Because size is of type uint32 → exactly 4 bytes are read.

	var size uint32
	err = binary.Read(r, binary.BigEndian, &size) // 4-byte size
	if err != nil {
		return n, err
	}

	// So far:
	//	- 1 byte type
	// 	- 4 bytes size
	//	- Total n = 5

	n += 4

	// 4) Security check: Size should not be too large
	// 	- This is very important:
	// 		- Because uint32 can give very large numbers (up to about 4 billion)
	// 		- If the attacker says size = 4GB
	// 		- and you try to make([]byte, size) → you empty RAM and the program crashes (DoS)
	// 		- Then before creating the buffer, it checks that it does not exceed the allowed limit (for example 10MB).

	if size > MaxPayloadSize {
		return n, ErrMaxPayloadSize
	}

	// 5) Create a buffer exactly the size of size
	// 	- This means:
	// 		- Create a slice inside `m` with length `size` bytes
	// 		- Since `m` is a pointer, we need to change the actual value with `*m`.
	//		- For example, if size=3:
	// 			- *m becomes a 3-bit slice like [0 0 0]

	*m = make([]byte, size)

	// 6) Read the payload and put it into the slice
	// 	- Now it should read the actual size of bytes from the network and put it into `*m`
	// 	- o means:
	// 		- How many bytes were actually read

	o, err := r.Read(*m) // payload

	// And finally:
	// 	- `n` (header = 5 bytes) +
	// 	- `o` (payload) that we read

	return n + int64(o), err
}
