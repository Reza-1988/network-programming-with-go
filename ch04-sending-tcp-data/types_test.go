package ch04

import (
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
			// 	- continue means go to the next message.

			t.Errorf("value mismatch: %V, expected, actual")
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
