package ch03

import (
	"io"
	"net"
	"testing"
	"time"
)

// --- STEP 8 ---
// ## Implementing Deadlines
// Go’s network connection objects allow you to include deadlines for both read and write operations.
// Deadlines allow you to control how long network connections can remain idle, where no packets traverse the connection.
// 	- You can control the Read deadline by using the `SetReadDeadline` method on the connection object,
//	- control the Write deadline by using the `SetWriteDeadline` method,
//	- or both by using the `SetDeadline` method.
//	When a connection reaches its read deadline, all currently blocked and future calls to a network connection’s Read method immediately return a time-out error.
//	Likewise, a network connection’s Write method returns a time-out error when the connection reaches its write deadline.
//	Go’s network connections don’t set any deadline for reading and writing operations by default,
//	- meaning your network connections may remain idle for a long time.
//	- This could prevent you from detecting network failures, like an unplugged cable, in a timely manner,
//	- because it’s tougher to detect network issues between two nodes when no traffic is in flight.
//	The server in Listing 3-9 implements a deadline on its connection object.

// - Once the server accepts the client’s TCP connection, you set the connection’s read deadline (1).
// - Since the client won’t send data, the call to Read will block until the connection exceeds the read deadline.
// - After five seconds, Read returns an error, which you verify is a time-out (2).
//	- Any future reads to the connection object will immediately result in another time-out error.
//	- However, you can restore the functionality of the connection object by pushing the deadline forward again (3).
//	- After you’ve done this, a second call to Read succeeds. The server closes its end of the network connection, which initiates the termination process with the client. The client, currently blocked on its Read call, returns io.EOF (4) when the network connection closes. We typically use deadlines to provide a window of time during which the remote node can send data over the network connection. When you read data from the remote node, you push the deadline forward. The remote node sends more data, and you push the deadline forward again, and so on. If you don’t hear from the remote node in the allotted time, you can assume that either the remote node is gone and you never received its FIN or that it is idle.

// Listing 3-9: A server-enforced deadline terminates the network connection

func TestDeadline(t *testing.T) {
	// Step 1: Prepare the server
	//	- sync A channel to synchronize the server and client (like a "go now" signal)
	// 	- listener Creates a local TCP server with a random port
	sync := make(chan struct{})

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	// Step 2: Server in goroutine
	// 	- The server waits for a client to connect (`Accept`).
	// 	- When the client connects, conn is created.
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		// Server cleaning
		// 	- Once this goroutine finishes:
		// 		- The connection is closed
		// 		- The sync channel is closed so that if someone was waiting for it, it doesn't get stuck
		// 		- Note: This `close(sync)` will cause `<-sync` to never wait for the instance infinitely.
		defer func() {
			_ = conn.Close()
			close(sync) // read from sync shouldn't block due to early return
		}()

		// Step 3: Set a deadline for the connection
		// 	- That is: from now until the next 5 seconds
		// 	- for both Read and Write
		// 	- After 5 seconds, if `Read()` is still waiting → it will `timeout`.
		err = conn.SetDeadline(time.Now().Add(5 * time.Second)) // (1)
		if err != nil {
			t.Error(err)
			return
		}

		// Step 4: The server wants to read a byte, but no data comes.
		//	- The server says: “Give 1 byte”
		// 	- But the client hasn’t sent anything yet
		// 	- So the Read gets stuck… until:
		// 		- Either the data comes
		// 		- Or the deadline is reached
		// - Here, because the data doesn’t come → after 5 seconds, it gets an err timeout
		buf := make([]byte, 1)
		_, err = conn.Read(buf) // blocked until remote node sends data

		// Step 5: Check if the error was actually a timeout
		// 	- It says:
		// 		- Is this error of type `net.Error`?
		// 		- And does `Timeout()` return true for it?
		// 		- If not → test fails
		nErr, ok := err.(net.Error)
		if !ok || !nErr.Timeout() { // (2)
			t.Errorf("expected timeout error; actual: %v", err)
		}

		// Step 6: Signals to the client, “Now you write.”
		//	- Meaning:
		// 		- “I saw the timeout, now you can send data”
		// 		- This channel is just for coordination so that the client doesn't write prematurely.
		sync <- struct{}{}

		// Step 7: Sets the deadline again
		// 	- Because the previous deadline has passed and the previous Read has timed out
		// 	- Now it gives another 5 seconds so that the next Read has time.
		err = conn.SetDeadline(time.Now().Add(5 * time.Second)) // (3)
		if err != nil {
			t.Error(err)
			return
		}

		// Step 8: The server reads again and this time expects data to arrive.
		// 	- This time the client writes after receiving the token
		// 	- so the Read should succeed and err should be nil
		_, err = conn.Read(buf)
		if err != nil {
			t.Error(err)
		}
	}()

	// Step 9: Client Section
	//	- The client connects to the server address.
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Waits for the server to timeout:
	//	- This line causes:
	// 		- The client does not send anything until the server sees the first “timeout”
	<-sync

	// - Now it sends a byte.
	//	- This is the data that the second Read reads from the server.
	_, err = conn.Write([]byte("1"))
	if err != nil {
		t.Fatal(err)
	}

	// Step 10: The client wants to read from the server but the server doesn't send anything.
	// 	- The server has not done any Write
	// 	- And when the server goroutine finishes, defer conn.Close() is executed
	// 	- That is, the server closes the connection
	// 	- So when the client reads:
	// 		- It should get `io.EOF` (that is, “the other side has closed the connection”)
	buf := make([]byte, 1)
	_, err = conn.Read(buf) // blocked until remote node sends data
	if err != io.EOF {      // (4)
		t.Errorf("expected server termination; actual: %v", err)
	}
}
