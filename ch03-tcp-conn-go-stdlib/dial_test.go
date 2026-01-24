package ch03

import (
	"io"
	"net"
	"testing"
)

// --- STEP 2 ---

// ## Establishing a Connection with a Server
// From the client’s side, Go’s standard library net package makes reaching out and
// establishing a connection with a server a simple matter.
// Listing 3-3 is a test that demonstrates the process of initiating a TCP connection
// with a server listening to 127.0.0.1 on a random port.

// Listing 3-3: Establishing a connection to 127.0.0.1
func TestDial(t *testing.T) {
	// Create a listener on a random port.
	//	- You start by creating a listener on the IP address 127.0.0.1, which the client will connect to.
	//	  You omit the port number altogether, so Go will randomly pick an available port for you.
	listener, err := net.Listen("tcp", "127.0.0.1:") // Now you have a server listening on, for example, 127.0.0.1:54321.
	if err != nil {
		t.Fatal(err)
	}

	// This channel is just for reporting. `struct{}` means Nothing!, we just want to say: So-and-so is done.
	done := make(chan struct{})
	// You spin off the listener in a goroutine (1) so you can work with the client’s side of the connection later in the test.
	// The listener’s goroutine contains code like Listing 3-2’s for accepting incoming TCP connections in a loop,
	// spinning off each connection into its own goroutine. (We often call this goroutine a **handler**.
	//I’ll explain the implementation details of the handler shortly, but it will read up to 1024 bytes from the socket at a time and log what it received.)
	go func() { // (1)

		// What does this first defer say in the goroutine?
		//	- defer means: Don’t run it now; let this current function finish when it’s finished.
		// 	- So this line means: `defer func()`
		// 		- Define a small function (anonymous)
		// 		- Call it… but because you put defer, it won’t run for now, It will run when this goroutine (the same function we’re in) returns and finishes.
		//	- When the server goroutine finishes, that defer is executed and does this: `done <- struct{}{}`
		//		- Send an empty message to the done channel
		// 		- This message simply acts as a bell: “I’m done.”
		defer func() { done <- struct{}{} }()

		// Why did we use a goroutine and then a loop?
		//	- Because `Accept()` blocks and waits for a connection to come in.
		//	- If you didn't put this loop inside a goroutine:
		// 		- The test would get stuck on `Accept()` And you would never get to `net.Dial(...)` (the client)
		// 		  So no connection would come in! That is, a deadlock.
		// 	- So we put a goroutine so that the server would wait in the background, and the main test thread could create the client.
		for {
			// Every time a client connects, `Accept` returns a `conn`
			// If the listener is closed (`listener.Close()`), Accept usually returns an error and the loop exits and the goroutine terminates.
			conn, err := listener.Accept() // (2)
			if err != nil {
				t.Log(err)
				return
			}
			// Why do we create a new goroutine for each conn?
			//	- Because multiple clients may connect at the same time.
			//	- If the server starts reading right after accepting a connection and, it takes a long time, it will not be able to accept the next connection.
			// 	- So the common pattern:
			// 		- The loop only accepts
			// 		- Delegates each conn to a separate goroutine to handle
			go func(c net.Conn) {
				// When this goroutine finishes, this defer is executed,
				//	- the connection is closed, and a message is sent to the done channel, meaning this connection is closed or down.
				defer func() {
					_ = c.Close()
					done <- struct{}{}
				}()
				// What does buffer mean?
				// A buffer here means a temporary container for holding data that you read from the network.
				buf := make([]byte, 1024) // We create a 1024-byte array.
				for {
					// `c.Read(buf)` means:
					// 	- Put whatever data has come in, up to 1024 bytes, into buf”
					// 	- Read returns two things:
					// 		- n: how many bytes were actually read
					// 		- err: error (or EOF)
					n, err := c.Read(buf) // (4)
					if err != nil {
						// What does `io.EOF` mean?
						// 	- It means the other side closed the connection and "there is no more data".
						// 	- So if it's `EOF`, it's normal and we just return.
						if err != io.EOF {
							t.Error(err)
						}
						return
					}
					// e.g. If n = 5, then only buf[0:5] is valid, not all 1024.
					t.Logf("received: %q", buf[:n])
				}
			}(conn)
		}
	}()

	// - The standard library’s `net.Dial()` function is like the `net.Listen()` function in that it accepts a network
	//   like tcp and an IP address and port combination in this case, the IP address and port of the listener to which it’s trying to connect.
	// - You can use a hostname in place of an IP address and a service name, like http, in place of a port number.
	//   If a hostname resolves to more than one IP address, Go will attempt a connection to each one in order until a connection succeeds or
	//   all IP addresses have been exhausted.
	//   Since IPv6 addresses include colon delimiters, you must enclose an IPv6 address in square brackets.
	//		- For example, "[2001:ed27::1]:https" specifies port 443 at the IPv6 address 2001:ed27::1.
	//	- Dial returns a connection object (conn) and an error interface value(err).
	conn, err := net.Dial("tcp", listener.Addr().String())
	// Dial means "Client Connect"
	// 	- `listener.Addr().String()` gives the actual address of the listener, like 127.0.0.1:54321
	// 	- Dial means “connect to this address”
	// 	- This causes the `Accept()` above to be released and issue a conn to the server.
	if err != nil {
		t.Fatal(err)
	}

	// We send "hello" to the server (via TCP connection).
	// Write returns two outputs: the number of bytes written and an error.
	// Here, we ignored both (_ , _) to simplify the test.
	_, _ = conn.Write([]byte("hello"))

	// Now that you’ve established a successful connection to the listener, you initiate a graceful termination of the connection from the client’s side (8).
	// After receiving the FIN packet, the Read method (4) returns the `io.EOF` error,
	// indicating to the listener’s code that you closed your side of the connection. The connection’s handler (3) exits, calling the connection’s Close method on the way out.
	// This sends a FIN packet to your connection, completing the graceful termination of the TCP session.
	_ = conn.Close() // (8) Close the TCP connection from the client side so that the server (Read) can either get the remaining data
	<-done

	// Finally, you close the listener (9). The listener’s Accept method (2) immediately unblocks and returns an error.
	// This error isn’t necessarily a failure, so you simply log it and move on. It doesn’t cause your test to fail.
	// The listener’s goroutine (1) exits, and the test completes.
	_ = listener.Close() // (9)
	<-done

	// What are these two `<-done`s for?
	//	- First the client is closed, the handler goroutine on the server should finish and return a done message, then <-done is waiting for that.
	// 	- Then the listener is closed, the main accept loop goroutine should catch an error and finish and return a done message, then the second <-done is waiting for that.
}
