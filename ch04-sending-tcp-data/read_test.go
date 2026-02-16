package ch04

import (
	"crypto/rand"
	"io"
	"net"
	"testing"
)

// --- STEP 1 ---
// ## Reading Data into a Fixed Buffer
/*
- TCP connections in Go implement the `io.Reader` interface, which allows you to read data from the network connection.
   - To read data from a network connection, you need to provide a buffer for the network connection’s Read method to fill.
- The Read method will populate the buffer to its capacity if there is enough data in the connection’s receive buffer.
   - If there are fewer bytes in the receive buffer than the capacity of the buffer you provide,
   - Read will populate the given buffer with the data and return instead of waiting for more data to arrive.
   - In other words, Read is not guaranteed to fill your buffer to capacity before it returns.
   - Listing 4-1 demonstrates the process of reading data from a network connection into a byte slice.
- You need something for the client to read, so you create a 16MB payload of random data (1) —more data than the client can read
	- in its chosen buffer size of 512KB (3) so that it will make at least a few iterations around its for loop.
  	- It’s perfectly acceptable to use a larger buffer or a smaller payload and read the entirety of the payload in a single call to Read.
	- Go correctly processes the data regardless of the payload and receive buffer sizes.
- You then spin up the listener and create a goroutine to listen for incoming connections.
   - Once accepted, the server writes the entire payload to the network connection (2).
   - The client then reads up to the first 512KB from the connection (4) before continuing around the loop.
   - The client continues to read up to 512KB at a time until either an error occurs or the client reads the entire 16MB payload.
*/
// Listing 4-1: Receiving data over a network connection
// This test aims to show with a real example that:
//   - The server sends a very large data (16MB)
//   - The client reads it in pieces with a smaller buffer (512KB)
//   - And each time you read it, you may return any arbitrary value (up to the buffer limit), not necessarily filling the buffer.
//   - Test Story:
//   	- We create a local “server” (listener)
//   	- A server goroutine sends 16MB of random data when the client connects
//   	- The client connects and reads into a 512KB buffer with Read until the connection is closed.
func TestReadIntoBuffer(t *testing.T) {
	// 1) Creating a large payload (16MB)
	// 	- `1<<24` means 2 to the power of 24 → 16,777,216 bytes ≈ 16MB
	// 	- `rand.Read(payload)` fills this array with random bytes.
	// 	- Goal: To have a large data that we have to read several times.
	payload := make([]byte, 1<<24) // 16 MB (1)
	_, err := rand.Read(payload)   // generate a random payload
	if err != nil {
		t.Fatal(err)
	}

	// 2) Building the server: `net.Listen`
	// 	- means: Find a free port on the localhost (the system itself) and listen
	// 	- That `:` means, the system itself chooses a free port.
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// 3) Server goroutine: Accept connection and send data
	// 	- What's going on here?
	//		- `Accept()` waits for a client to connect.
	// 		- When it does, we get a conn (server-side connection).
	// 		- Then `conn.Write(payload)` tries to send the entire 16MB over the network.
	// 	- Note: Write doesn't necessarily send the entire 16MB "in one packet"; TCP breaks it up into chunks.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		defer conn.Close()

		_, err = conn.Write(payload) // (2)
		if err != nil {
			t.Error(err)
		}
	}()

	// 4) Client connects with `net.Dial`
	// 	- This is a client-side connection.
	// 	- The address it connects to is the listener (server) address.
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	// 5) Create a 512KB buffer for reading
	// 	- `1<<19` means 2 to the power of 19 → 524,288 bytes ≈ 512KB
	// 	- The client buffer is much smaller than the payload.
	// 	- So we have to read a few times.
	buf := make([]byte, 1<<19) // 512 KB (3)

	// 6) Read loop until the connection is complete
	// 	- Here is the most important part of the test:
	// 		- What does `conn.Read(buf)` do?
	// 			- Each time it tries to put “how much data is ready now” into buf.
	// 			- `n` tells how many bytes were actually filled.
	// 			- It could be:
	// 				- n = 512KB (if a lot of data is ready) or less (say 200KB) if less is ready
	// 	- And this is exactly the point of the book:
	// 		- Read is not guaranteed to fill the buffer completely.
	// 	- `conn.Read(buf)` according to the `io.Reader` contract is as follows:
	// 		- You prepare a buffer
	// 		- Read just fills that buffer
	// 		- And says how many bytes were filled (n)
	// 		- That is, Read has no right to change the length of the slice or decide to make it bigger. Because:
	// 			- It is just a “Reader”, not a “memory builder”.
	// 			- Memory management should be in your hands (so that you have control and the program does not eat RAM unnecessarily).
	// 		- So here buf is like a “fixed container”, not a growing list.
	for {
		n, err := conn.Read(buf) //(4)
		// 7) Why does the loop finally end?
		// 	- When the server is done and `conn.Close()` is executed (due to defer conn.Close() on the server), the connection is closed.
		// 	- Then on the client side:
		// 		- It gives Read or `io.EOF`
		// 		- That means: “No more data, the other side closed the connection”
		// 	- This code says:
		//		- If the error was EOF → normal → loop complete
		// 		- If it was another error → real problem → error log
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}
		// 8) What does buf[:n] mean?
		// 	- When you read, only the 0 to n-1 parts of buf are new data.
		// 	- The rest of the buffer may already have something in it or be zero.
		// 	- So the "real data" each round is: buf[:n]
		t.Logf("read %d bytes", n) // buf[:n] is the data read from conn
	}

	// 9) Finally, the client closes the connection.
	// 	- (It would have been better to also put defer conn.Close(), but that's okay.)
	_ = conn.Close()
}
