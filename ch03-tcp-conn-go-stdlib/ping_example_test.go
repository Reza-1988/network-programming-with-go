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
// 	- In this example, you create a buffered channel (1) that you’ll use to signal a reset of the Pinger’s timer.
//		- You put an initial ping interval of one second on the `resetTimer` channel before passing the channel to the Pinger function.
//		- You’ll use this duration to initialize the Pinger’s timer and dictate when to write the ping message to the writer.
//	- You run through a series of millisecond durations in a loop (2), passing each to the receivePing function.
//		- This function resets the ping timer to the given duration and then waits to receive the ping message on the given reader.
//		- Finally, it prints to stdout the time it takes to receive the ping message. Go checks stdout against the expected output in the example.
// 	- During the first iteration (3), you pass in a duration of zero,
//		- which tells the Pinger to reset its timer by using the previous duration—one second in this example.
//		- As expected, the reader receives the ping message after one second.
//	- The second iteration (4) resets the ping timer to 200 ms. Once this expires, the reader receives the ping message.
//	-The third run resets the ping timer to 300 ms (5), and the ping arrives at the 300 ms mark.
// 	- You pass in a zero duration for run (4) (6), preserving the 300 ms ping timer from the previous run.
//		- I find the technique of using zero durations to mean “use the previous timer duration” useful because I do not need to keep track of the initial ping timer duration.
//		- I can simply initialize the timer with the duration I want to use for the remainder of the TCP session and
//		- reset the timer by passing in a zero duration every time I need to preempt the transmission of the next ping message.
//		- Changing the ping timer duration in the future involves the modification of a single line as opposed to every place I send on the resetTimer channel.
// 	- Runs 5 to 7 (7) simply listen for incoming pings without resetting the ping timer.
//		- As expected, the reader receives a ping at 300 ms intervals for the last three runs.
//	- With Listing 3-10 saved to a file named ping.go and Listing 3-11 saved to a file named ping_example_test.go, you can run the example by executing the following:
//		- $ go test ping.go ping_example_test.go

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
	// 4) Making the resetTimer channel buffered
	// 	- This channel is the “timer control”.
	// 	- Buffer 1 means:
	// 		- You can put a value in it even if no one is ready to read it yet.
	resetTimer := make(chan time.Duration, 1)
	// 	- This line means:
	//		- initial ping interval = 1 second
	resetTimer <- time.Second // initial ping interval (1)

	// 5) Running Pinger in a goroutine
	// 	- This goroutine:
	// 		- Runs Pinger
	// 		- When Pinger finishes (with cancel or error), it closes done to let others know it's finished.
	go func() {
		Pinger(ctx, w, resetTimer)
		close(done)
	}()

	// 6) `receivePing` function (the name is misleading)
	// 	- It's named `receivePing` but it actually does this:
	// 		- If d >= 0:
	//			- Prints a message
	// 			- Sends the new interval to the resetTimer channel
	// 			- That is: "Reset the ping timer to this value"
	// 	- Note:
	// 		- The r parameter of `io.Reader` is not used at all here! (probably an addition/leftover from a previous version)

	receivePing := func(d time.Duration, r io.Reader) {
		if d >= 0 {
			fmt.Printf("resetting timer (%s)\n", d)
			resetTimer <- d
		}

		// 7) Read the first ping and measure its time
		// 	- `now := time.Now()` → Start time
		// 	- `r.Read(buf)` waits for something to come from the pipe
		//		- The same "ping" that Pinger wrote on w
		// 	- `n` is the number of bytes read
		// 	- `buf[:n]` is the actual data itself
		//	- time.Since(now) means:
		// 		- How long did it take for this ping to arrive
		// 		- `Round(100*time.Millisecond)` is just to make the time printout prettier.
		now := time.Now()
		buf := make([]byte, 1024)
		n, err := r.Read(buf)
		if err != nil {
			fmt.Print(err)
		}
		fmt.Printf("received %q (%s)\n",
			buf[:n], time.Since(now).Round(100*time.Millisecond))
	}
	// 8) What does this loop do?
	// 	- Here is a list of times (in milliseconds):
	// 		- 0ms, 200ms, 300ms, 0ms, -1ms, -1ms, -1ms
	// 	- For each:
	// 		- Prints Run time
	// 		- Converts v to duration: `time.Duration(v) * time.Millisecond`
	// 	- Then sends to `receivePing`
	// 		- ReceivePing rule:
	// 			- If d >= 0 → new interval is sent
	// 			- If d < 0 → does nothing (does not reset)
	// 	- So:
	// 		- 0 → reset to 0 (this is logically strange; because in Pinger if interval <=0 it goes to default 30s)
	// 		- 200 → reset to 200ms
	// 		- 300 → reset to 300ms
	// 		- 0 → <=0 again
	// 		- -1 → ignore
	// 		- -1 → ignore
	// 		- -1 → ignore
	// 		- Author's goal:
	// 			- Show:
	// 				- How Pinger changes/ignores the interval when you send different values
	for i, v := range []int64{0, 200, 300, 0, -1, -1, -1} { // (2)
		fmt.Printf("Run %d:\n", i+1)
		receivePing(time.Duration(v)*time.Millisecond, r)
	}

	// 9) Turn off Pinger
	// 	- cancel() signals a stop
	// 	- `<-done` means:
	// 		- Wait until the goroutine actually finishes and done closes
	//	- This prevents the goroutine from "leaking".
	cancel()
	<-done // ensures the pinger exits after canceling the context

	// Output:
	// Run 1:
	// resetting timer (0s)
	// received "ping" (1s)
	// Run 2:
	// resetting timer (200ms)
	// received "ping" (200ms)
	// Run 3:
	// resetting timer (300ms)
	// received "ping" (300ms)
	// Run 4:
	// resetting timer (0s)
	// received "ping" (300ms)
	// Run 5:
	// received "ping" (300ms)
	// Run 6:
	// received "ping" (300ms)
	// Run 7:
	// received "ping" (300ms)

}
