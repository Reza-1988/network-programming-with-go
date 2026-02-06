package ch03

import (
	"context"
	"fmt"
	"io"
	"time"
)

// --- STEP 8 ---
// Listing 3-11 illustrates how to use the Pinger function introduced in Listing 3-10 by giving it a writer and running it in a goroutine.
// You can then read pings from the reader at the expected intervals and reset the ping timer with different intervals.

// Listing 3-11: Testing the pinger and resetting its ping timer interval
// 0) What is the purpose of the example?
//   - We have a goroutine that writes "ping" every few seconds.
//   - We also read on the other side when the ping arrived.
//   - Then we say by sending a new value to the resetTimer channel:
//   - “Ping every X milliseconds from now on”
func ExamplePinger() {
	// 1) Creating a context to turn off Pinger
	// 		- ctx = Common Control
	// 	- cancel() = Off key → Whenever you call it, Pinger should stop.
	ctx, cancel := context.WithCancel(context.Background())
	// 2) What does `io.Pipe()` mean? (Very important)
	// 	- io.Pipe() creates a “pipe” in memory:
	// 		- Whatever you write to `w` (Writer),
	// 		- you can read from `r` (Reader).
	// 	- It’s like creating a “fake network connection”, but completely inside the program and without TCP.
	// 	- Why did it do this?
	// 		- Because Pinger only wants an io.Writer.
	// 		- It doesn’t have to be a net.Conn.
	// 		- So it’s easy to test with Pipe.
	r, w := io.Pipe() // in lieu of net.Conn
	// 3) The done channel to let us know when the goroutine has finished.
	//	- This channel is just to wait until Pinger is actually down.
	done := make(chan struct{})
	resetTimer := make(chan time.Duration, 1)
	resetTimer <- time.Second // initial ping interval (1)

	go func() {
		Pinger(ctx, w, resetTimer)
		close(done)
	}()

	receivePing := func(d time.Duration, r io.Reader) {
		if d >= 0 {
			fmt.Printf("resetting time (%s)\n", d)
			resetTimer <- d
		}
	}

	now := time.Now()
	buf := make([]byte, 1024)
	n, err := r.Read(buf)
	if err != nil {
		fmt.Print(err)
	}
	fmt.Printf("received %q (%s)\n",
		buf[:n], time.Since(now).Round(100*time.Millisecond))

	for i, v := range []int64{0, 200, 300, 0, -1, -1, -1} {
		fmt.Printf("Run %d:\n", i+1)
		receivePing(time.Duration(v)*time.Millisecond, r)
	}

	cancel()
	<-done // ensures the pinger exits after canceling the context

}
