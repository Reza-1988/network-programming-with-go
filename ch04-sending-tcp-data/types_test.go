package ch04

import (
	"net"
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

}
