package ch03

import (
	"io"
	"net"
	"testing"
	"time"
)

// --- STEP 3 ---

// ## Understanding Time-outs and Temporary Errors
// In a perfect world, your connection attempts will immediately succeed, and all read and write attempts will never fail.
// But you need to hope for the best and prepare for the worst.
// You need a way to determine whether an error is temporary or something that warrants termination of the connection altogether.
// The error interface doesn’t provide enough information to make that determination.
// Thankfully, Go’s net package provides more insight if you know how to use it.

// Errors returned from functions and methods in the net package typically implement the `net.Error` interface, which includes two notable methods:
// - Timeout and Temporary.
//	- `Timeout()`
//		- The `Timeout()` method returns true on Unix-based operating systems and Windows if the operating system tells Go
//		- that the resource is temporarily unavailable, the call would block, or the connection timed out.
//		- We’ll touch on time-outs and how you can use them to your advantage a bit later in this chapter.
//	- `Temporary()`
//		- The Temporary method returns true if the error's Timeout function returns true, the function call was interrupted,
//		- or there are too many open files on the system, usually because you’ve exceeded the operating system’s resource limit.

// When do we use this? Listener or Connection?
// 	-1) When accepting connections (server)
//		- When you have a server and, you loop for accept connection: `conn, err := listener.Accept()`
// 		- If Accept returns an error:
// 			- Some errors are temporary (e.g. the system is currently out of resources)
//
//	-2) When Read/Write on conn
// 		- Here too, the error can be temporary/timeout: `n, err := conn.Read(buf)`
// 		- If it was a timeout, you might want to:
// 			- Try again
// 			- Or give an appropriate message
// 			- Or change the timing

// Why do we need “type assertion”?
// 	- Because `net` functions say error in their signature, not `net.Error`.
// 	- So you need to check if this err is really `net.Error` or not:
// 		- If err was of type net.Error and was not temporary
// 			- then this error is serious → ignore / return / abort
// 		- but if it was temporary:
// 			- you can retry

// Listing 3-4: Asserting a net.Error to check whether the error was temporary
func TestDialTemporary(t *testing.T) {

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {

		defer func() { done <- struct{}{} }()

		for {
			conn, err := listener.Accept()
			if err != nil {
				// 1) `Accept()` error (when the server is waiting for a new connection), simple meaning:
				// 	- When `listener.Accept()` returns an error, we ask:
				// 		- Is this error a “network error” (`net.Error`)? `nErr, ok := err.(net.Error)`
				// 			- `ok == true` means yes, this error has `Temporary()` and `Timeout()` capabilities.
				// 			- `If ok == false`, this error has no additional network information.
				// 		- If it was `net.Error`, is it temporary? `nErr.Temporary()` Means:
				// 			- This problem can probably be fixed with a little patience?
				// 			- For example, the system is under pressure for a moment, resources are low, a temporary problem.
				// 		- If it was temporary, what do we do?
				// 			- We don't kill the server
				// 			- We just log and, We wait a bit (50ms) until the CPU is not 100%
				// 			- Then we go to the beginning of the loop with `continue` and again `Accept()`
				// 			- Result: The server becomes "resistant" and does not die with a short error.
				if nErr, ok := err.(net.Error); ok && nErr.Temporary() {
					// The error is temporary => the server won't die, it will try again
					t.Logf("temporary accept error: %v", err)
					time.Sleep(50 * time.Microsecond) // Prevent busy loop
					continue
				}
				// The error is not temporary => usually means the listener is closed or a serious error
				t.Logf("accept stopped: %v", err)
				return
			}

			go func(c net.Conn) {
				defer func() {
					c.Close()
					done <- struct{}{}
				}()
				buf := make([]byte, 1024)
				for {

					n, err := c.Read(buf)
					if err != nil {
						// 2) Read() error (when reading from a data connection). simple meaning:
						// 	- First we ask:
						// 		- Is this Read error of type `net.Error`?
						// 		- If it is, we check two important conditions:
						//			- Mode A: Timeout, Timeout means:
						//				- I waited too long for data and nothing came. Here we decided:
						// 				- Let’s consider this problem “normal/tolerable”
						//				- So we don’t disconnect
						// 				- We just go to the next round of the loop and read again (continue)
						// 				- Important note: This usually happens when you have set a deadline yourself. Otherwise, you might see less.
						//			- Mode B: Temporary. Temporary means:
						//				- It’s a temporary problem, maybe you can try again now and, it’ll be fixed. So:
						// 				- We log and, We wait a bit (10ms)
						// 				- We read again (continue)
						// 				- Result: Instead of getting disconnected with a temporary error, we try again.
						if nErr, ok := err.(net.Error); ok {
							// Mode A
							if nErr.Timeout() {
								t.Logf("read timeout (will continue): %v", err)
								continue
							}
							// Mode B
							if nErr.Temporary() {
								t.Logf("temporary read error (will continue): %v", err)
								time.Sleep(10 * time.Millisecond)
								continue
							}
						}
						// If it was neither Timeout nor Temporary (or not net.Error at all)
						// 	- What does io.EOF mean?
						//		- EOF means:
						// 			- The other party has closed the connection and there is no more data.
						// 			- This is normal (the client has closed), so we don’t count it as an error and just exit the handler.
						// 		- What if there was no EOF?
						// 			- That means there was a more real problem:
						// 				- For example, a connection reset
						// 				- or a network outage
						// 				- or another serious error
						// 				- So: `t.Error(err)`
						// 					- We fail the test (but we don’t immediately terminate like `Fatal`)
						//				- And then: return
						// 					- We exit the goroutine related to this connection (meaning we’re done handling this client).

						if err != io.EOF {
							t.Error(err)
						}
						return
					}

					t.Logf("received: %q", buf[:n])
				}
			}(conn)
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())

	if err != nil {
		t.Fatal(err)
	}

	_, _ = conn.Write([]byte("hello"))
	_ = conn.Close()
	<-done

	_ = listener.Close()
	<-done
}
