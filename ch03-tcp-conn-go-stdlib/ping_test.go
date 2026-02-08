package ch03

import (
	"context"
	"io"
	"net"
	"testing"
	"time"
)

// --- STEP 10 ---
// ## Advancing the Deadline by Using the Heartbeat
// Each side of a network connection could use a Pinger to advance its deadline if the other side becomes idle,
// whereas the previous examples showed only a single side using a Pinger.
// When either node receives data on the network connection, its ping timer should reset to stop the delivery of an unnecessary ping.
// Listing 3-12 is a new file named ping_test.go that shows how you can use incoming messages to advance the deadline.

// Listing 3-12: Receiving data advances the deadline
// This test is intended to prove very simply:
//   - The server has deadline = 5s (if no data comes in for 5 seconds, the connection will be closed/timed out).
//   - But because the server sends a heartbeat (Pinger), the connection stays “alive”.
//   - And whenever the server receives any data (even its own “ping” or a “PONG!!!” from the client),
//   - it both advances the deadline by 5 seconds
//   - and resets the ping timer so that it doesn’t send any additional pings.
//   - Finally, it is set up so that the connection will end exactly after about 9 seconds and give EOF.
//   - Roles:
//   - Server: goroutine inside go func(){...} that accepts and executes Pinger.
//   - Client: below function that dials and reads pings and sends PONG!!! once.
func TestPingerAdvanceDeadline(t *testing.T) {
	// A) Server part (goroutine)
	// A-1) Preparation
	// 	- `done` is to let us know that the server is finished.
	// 	- `begin` is to tell the logs how many seconds have passed.
	done := make(chan struct{})
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	begin := time.Now()
	// A-2) The server starts and waits for a connection.
	// 	- Here the server waits for the client to connect.
	go func() {
		defer func() { close(done) }()
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		// A-3) Context to turn off Pinger
		// 	- The server creates a `ctx` to stop Pinger when it is done.
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			cancel()
			_ = conn.Close()
		}()

		// A-4) 4) Create resetTimer and run Pinger
		// 	- `resetTimer` is buffered so that the initial value is not stuck.
		// 	- Initial value: 1 second → means that the pinger writes a "ping" to conn approximately every 1 second.
		// 	- `Pinger` works with writer = `conn`, so the pings are actually sent over the TCP connection.
		resetTimer := make(chan time.Duration, 1)
		resetTimer <- time.Second
		go Pinger(ctx, conn, resetTimer)

		// A-5) Set an initial deadline for the connection
		//	- That means: From this moment on, if no successful Read/Write occurs within 5 seconds, the operations will timeout.
		err = conn.SetDeadline(time.Now().Add(5 * time.Second))
		if err != nil {
			t.Error(err)
			return
		}

		// A-6) Loop reading data from the connection (The server only reads what the client sends to the server (here just PONG!!!))
		// 	- This part is very important. What it means:
		// 		- The server reads constantly and prints whatever it receives.
		// 		- Every time “any data” arrives (for example, “ping” or “PONG!!!”):
		// 	1. `resetTimer <- 0`
		// 		- That is: “Reset the ping timer but do not change the interval”
		// 		- Because Pinger previously had interval = 1s
		// 		- So with 0 it just starts counting again
		// 		- Result: If the next ping is about to be sent, it will fall behind (the extra ping will be subtracted)
		// 	2. `SetDeadline(now + 5s)`
		// 		- That is: “From now on, give it 5 more seconds”
		// 		- So since messages arrive regularly, the deadline will always advance and the connection will stay alive.
		// Note: In this test, even the pings that the server itself sends will cause the other party to read something and
		// then the cycle will continue again (and the deadline will not fall behind).
		buf := make([]byte, 1024)
		for {
			// It will remain blocked until the client sends anything.
			// After waiting a few seconds, the client sends a message to the server to push the deadline forward.
			n, err := conn.Read(buf)
			if err != nil {
				return
			}
			t.Logf("[%s] %s",
				time.Since(begin).Truncate(time.Second), buf[:n])
			// `Truncate(time.Second)`:
			//	- That is, it does not round the past time, but rounds it down to the nearest second.
			//	- Example:
			// 		- 4.9s → becomes 4s
			// 		- 4.1s → becomes 4s
			// 	- Why is it used?
			// 		- Because times fluctuate to the exact millisecond, but the author wants the logs to be “seconds” and stable.

			resetTimer <- 0

			// When the server receives a data (for example PONG!!!) it writes this line:
			// 	- SetDeadline(now + 5s)
			//	- This means that the deadline will be set back 5 seconds from now.
			err = conn.SetDeadline(time.Now().Add(5 * time.Second))
			if err != nil {
				t.Error(err)
				return
			}
		}
	}()

	// B) Client section
	// B-7) Connecting to the server
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// B-8) Read the first 4 pings
	// 	- Since the pinger sends a ping every 1 second,
	// 	- the client receives four "pings" at approximately 1,2,3,4 seconds.
	// 	- What is the goal?
	//		- It wants the client to do something for about 4 seconds and then send a pong to push the deadline to 9 seconds.
	buf := make([]byte, 1024)
	for i := 0; i < 4; i++ { // read up to four pings
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("[%s] %s", time.Since(begin).Truncate(time.Second), buf[:n])
	}

	// B-9) The client sends “PONG!!!” once
	// This message is supposed to indicate that “when the actual data arrives from the client”:
	// 	- The server reads it
	// 	- And as before:
	// 		- Resets the ping timer
	// 		- Advances the deadline by 5 seconds
	_, err = conn.Write([]byte("PONG!!!")) // should reset the ping timer
	if err != nil {
		t.Fatal(err)
	}

	// B-10) Then it reads again for another 4 pings (or until EOF)
	//	- After PONG!!! it expects a few more pings to arrive,
	// 	- but eventually the connection is closed and Read on the client returns EOF.
	// 	- But why might we get EOF in the middle?
	// 		- Because after receiving PONG, the server goes back to Read and waits for the next data from the client.
	// 		- But the client doesn't send anything anymore.
	// 		- So 5 seconds after PONG, the server's deadline arrives → the Read server gets an error → the server goroutine returns → the connection is closed → the client gets EOF.
	for i := 0; i < 4; i++ { // read up to four more pings
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				t.Fatal(err)
			}
			break
		}
		t.Logf("[%s] %s", time.Since(begin).Truncate(time.Second), buf[:n])
	}

	// C) End of test: Why does it expect it to take exactly 9 seconds?
	// C-11) Waits for the server to finish
	// 	- The idea of the timing is roughly this:
	// 		- The pinger sends a ping every 1 second, so for the first few seconds we have constant traffic → the deadline advances.
	// 		- After a while (with resets and read/write ordering) the last “meaningful activity” occurs such that:
	//			- About 5 seconds later there is not enough traffic anymore
	// 			- and the server deadline arrives
	// 			- The server Read gets an error and returns → the connection is closed
	// 			- The client also gets EOF
	// 	- The author expects this moment to occur around second 9.
	// 	- If this test fluctuates a bit on busy/slow systems, it may become flaky; but since it does Truncate(time.Second), it is only sensitive to seconds.
	<-done // Wait for the server to actually finish
	end := time.Since(begin).Truncate(time.Second)
	t.Logf("[%s] done", end)
	if end != 9*time.Second {
		t.Fatalf("expected EOF at 9 seconds; actual %s", end)
	}
}
