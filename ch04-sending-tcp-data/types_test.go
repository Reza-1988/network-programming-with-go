package ch04

import (
	"bytes"
	"encoding/binary"
	"net"
	"reflect"
	"testing"
)

// Listing 4-10: Creating the TestPayloads test
// 	- Your test should first create at least one of each type.
//		- You create two Binary types (1) and one String type (2).
//		- Next, you create a slice of Payload interfaces and add pointers to the Binary and String types you created (3).
//		- You then create a listener that will accept a connection and write each type in the payloads slice to it (4).

// This part of the test only creates the "first half":
//	- it creates some payloads and starts a server that writes these payloads one by one to the connection when the client connects.

func TestPayloads(t *testing.T) {

	// 1) Creating multiple payloads of `Binary` and `String` types
	// 	- `Binary` is defined in your code: type `Binary []byte`
	// 		- When you write `Binary("...")` it means it converts that text to bytes and stores it in Binary format (binary payload)
	// 		- `String` is also defined: type `String string`
	// 		- s1 is a text payload.
	// 	- So now we have 3 messages:
	// 		- `b1 (Binary)`
	// 		- `b2 (Binary)`
	// 		- `s1 (String)`

	b1 := Binary("Clear is better that clever.") // (1)
	b2 := Binary("Don't panic.")                 // (2)
	s1 := String("Errors are values.")

	// 2) Creating a list of payloads in the form of an interface
	// 	- `Payload` is an interface that has methods like `WriteTo` and `ReadFrom`.
	// 	- Here is a slice of `Payload` created.
	// Why `&b1` and `&s1` and `&b2` (address)?
	// 	- Because `ReadFrom` is written with a pointer receiver for each type, and usually they want to always work with pointers to be able to fill/modify.
	// 	- Even if they only call `WriteTo` here, it remains consistent and uniform by putting pointers.
	// 	- Result: payloads is a list of “things that behave like Payloads”.

	payloads := []Payload{&b1, &s1, &b2} // (3)

	// 3) Creating a server (listener)

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// 4) Server goroutine: Accept connections and write payloads
	// 	- What happens inside the goroutine?
	// 		- `Accept()` waits for a client to connect.
	// 		- When the client connects, it gets a connection (conn).
	// 		- Then it loops through the payloads:
	//			- It writes each payload to the connection with `p.WriteTo(conn)`
	// 		- What goes on the network?
	// 			- For each payload, the same TLV is written:
	// 				- 1 byte type
	//				- 4 bytes length
	// 				- payload bytes
	// 				- So this server sends 3 TLV messages in a row:
	// 					- b1, s1, b2
	// 				- If an error occurs while writing:
	// 					- It throws `t.Error(err)` and breaks (it doesn't continue any further)

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()

		for _, p := range payloads {
			_, err = p.WriteTo(conn) // (4)
			if err != nil {
				t.Error(err)
				break
			}
		}
	}()

	// Listing 4-11: Completing the TestPayloads test
	// 	- You know how many types to expect in the payloads slice, so you initiate a connection to the listener (1) and attempt to decode each one (2).
	//	- Finally, your test compares the type you decoded with the type the server sent (3).
	//		- If there’s any discrepancy with the variable type or its contents, the test fails.
	//		- You can run the test with the -v flag to see the type and its value 4.

	// This is the “second half” of the test: the client connects, reads messages one by one over the connection, and compares them with what the server sent.

	// 1) The client connects to the server
	// 	- As before: the client connects to the listener.
	// 	- Now `conn` is the stream of bytes into which the server has written the TLVs one after the other.

	conn, err := net.Dial("tcp", listener.Addr().String()) // (1)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// 2) We loop because we know how many messages we expect.
	// 	- `payloads` is the expected list: `&b1, &s1, &b2`
	// 	- So we know we have to read exactly 3 messages in a row from the network.
	// 	- So we `decode` 3 times.

	for i := 0; i < len(payloads); i++ {

		// 3) Each time `decode(conn)` reads a complete message TLV
		// 	- `decode` reads from the connection:
		// 		- First 1 byte type
		// 		- Then 4 bytes length
		//		- Then payload
		// 		- And the result returns a value of type Payload:
		// 			- Either *Binary Or *String
		// 	- We named it `actual` because it's "what we actually got from the network".

		actual, err := decode(conn) // (2)
		if err != nil {
			t.Fatal(err)
		}

		// 4)
		// 	- What exactly does this do?
		// 		- This `if` does two things at once:
		// 			- A) Creates a temporary variable: `expected := payloads[i]`
		//				- That is: “The expected message in this loop round”
		// 					- For example:
		//						- in the first round: `expected = &b1`
		// 						- In the second round: `expected = &s1`
		//						- In the third round: `expected = &b2`
		// 				- This expected variable can only be used inside this if (limited scope).
		//			- B) compares whether `expected` and `actual` are the same.
		//				- `!reflect.DeepEqual(expected, actual)`
		//				- `reflect.DeepEqual` means “deep” comparison
		//					- It checks for both type and content
		// 						- So here it checks:
		//							- Is what the server sent (expected)
		// 							- exactly the same as what we decoded (actual)
		// 							- Is it exactly the same?
		// 								- If they are not the same → it enters the if body and tests for error.

		if expected := payloads[i]; !reflect.DeepEqual(expected, actual) { // (3)

			// 5) What would she do if it were different?
			// 	- Meaning: “This message was different”
			//		- `t.Errorf(...)`
			//			- Records an error in the test (the final test fails)
			// 			- But unlike `t.Fatal`, it doesn't abort the test immediately; it just reports the error and continues.
			// 			- `%v` means print the expected and actual values as default.
			// 				- Since `expected` and `actual` are of type `Payload` and `Payload` contains `fmt.Stringer`,
			// 				- `%v` usually prints the output of `String()` (i.e. the text inside the message).
			// 	- continue
			// 		- Means: "Skip this message, go to the next message in the loop"
			// 		- This helps us check subsequent messages if the first message is broken and see multiple errors at once, rather than stopping the test right away.

			t.Errorf("value mismatch: %v != %v", expected, actual)
			continue
		}

		// 6) If equal, it takes the log
		// 	- `%T` means print the variable type
		// 	- 	- For example:
		// 		- `*ch04.Binary` or `*ch03.String`
		// 		- So the output is like: [*ch04.Binary] ...
		// 	- What does `%[1]q` mean?
		// 		- `[1]` means “use the first argument” (the same as actual)
		// 		- `q` means “print with quotes” (like "...")
		// 		- So this line means:
		// 			- Print the type actual
		// 			- And then print actual itself in quoted form
		// 		- Why is actual printable with `%q`?
		// 			- Because the Payload contains `fmt.Stringer` and has `String()`,
		// 			- Go can convert actual to a string and print it.

		t.Logf("[%T] %[1]q", actual) // (4)
	}

}

// Listing 4-12: Testing the maximum payload size
// 	- This test starts with the creation of a `bytes.Buffer` containing the `BinaryType` byte and a 4-byte,
//	  unsigned integer indicating the payload is 1GB (1).
//	-  When this buffer is passed to the Binary type’s `ReadFrom` method, you receive the `ErrMaxPayloadSize` error in return (2).
//	- The test cases in Listings 4-10 and 4-11 should cover the use case of a payload that is less than the maximum size,
//	- but I encourage you to modify this test to make sure that’s the case.

// This test wants to make sure that the `Binary.ReadFrom` code stops before it tries to create a large buffer when the message length is too large and throws an `ErrMaxPayloadSize` error.
// 	- What is the purpose of the test?
// 		- We said in the TLV protocol:
//  		- 1 byte Type
// 			- 4 bytes Length
// 			- Then payload
// 			- And we said that if Length was greater than `MaxPayloadSize`, we should throw an error `(ErrMaxPayloadSize`).
// 			- This test checks exactly that.

func TestMaxPayloadSize(t *testing.T) {

	// 1) We create a buffer in memory (like a fake network connection)
	// 	- `bytes.Buffer` is like a container that you can put bytes into
	// 	- and then read from like `io.Reader`
	// 	- Here it plays the role of a "network/connection", but it is not real.

	buf := new(bytes.Buffer)

	// 2) First we write the Type into the buffer (1 byte)
	// 	- This is the Type TLV
	// 	- That is, we are saying: “This message is of type Binary”
	// 	- `BinaryType` is a `uint8`, so exactly one byte is written.

	err := buf.WriteByte(BinaryType)
	if err != nil {
		t.Fatal(err)
	}

	// 3) Next we write Length (4 bytes) and make this length very large.
	// 	- This line means:
	// 		- `1<<30` means 2 to the power of 30 = 1,073,741,824 bytes ≈ 1GB
	// 		- Converting to uint32 means we want to write it in 4 bytes
	// 		- `binary.Write` puts those 4 bytes in `BigEndian` order into `buf`
	// 		- So what do we have in buf now?
	// 		- [Type = BinaryType (1 byte)] [Length = 1GB (4 bytes)]
	// 	- Note: We don't write the actual 1GB payload at all,
	// 		- because the purpose of the test is for the program to understand that this length is unacceptable before creating the buffer.

	err = binary.Write(buf, binary.BigEndian, uint32(1<<30)) // 1GB (1)
	if err != nil {
		t.Fatal(err)
	}

	// 4) We create an empty binary.
	// 	- `b` is currently nil/empty and was supposed to be filled with `ReadFrom`.

	var b Binary

	// 5) Now we say read the TLV from this buffer.
	// 	- `ReadFrom` first reads 1 byte of type → `BinaryType` (accepted)
	// 	- Then reads 4 bytes of size → 1GB
	// 	- Then it comes to this security check in ReadFrom:
	// 		- If size > `MaxPayloadSize` → `ErrMaxPayloadSize` error
	// 		- Then the exact same error should be returned here.

	_, err = b.ReadFrom(buf)

	// 6) We check whether the exact same error is returned or not.
	// 	- If the error was not `ErrMaxPayloadSize`:
	// 		- The test fails and tells you what the error was.
	// 		- This means the test will only succeed if:
	// 			- `ReadFrom` actually checked the size
	// 			- and stopped before allocating the large buffer.

	if err != ErrMaxPayloadSize { // (2)
		t.Fatalf("expected ErrMaxPayloadSize; actual: %v", err)
	}
}
