package ch03

import (
	"context"
	"net"
	"syscall"
	"testing"
	"time"
)

// --- STEP 6 ---
// ## Aborting a Connection by Canceling the Context
//	- Another advantage to using a context is the cancel function itself.
// 	- You can use it to cancel the connection attempt on demand, without specifying a deadline, as shown in Listing 3-7.

// Listing 3-7: Directly canceling the context to abort the connection attempt
func TestDialContextCancel(t *testing.T) {
	// Instead of creating a context with a deadline and waiting for the deadline to abort the connection attempt,
	// you use `context.WithCancel` to return a context and a function to cancel the context (1)
	// 1) Creating a cancelable context: `ctx, cancel := context.WithCancel(context.Background())` Simple meaning:
	// 	- We create a “cancel button”
	// 	- Any task started with ctx should be stopped by pressing this button
	//	- Here:
	// 		- We don’t have a deadline
	//		- We only have manual cancellation
	ctx, cancel := context.WithCancel(context.Background()) // (1)

	// 2) Create a channel for synchronization (sync): `sync := make(chan struct{})`, Simple meaning:
	// 	- We create a bell/signal to notify us when the goroutine has finished its work
	// 	- Purpose:
	// 		- The test does not end early and waits for the connection attempt inside the goroutine to finish
	sync := make(chan struct{})

	// Since you’re manually canceling the context, you create a closure and spin it off in a goroutine to handle the connection attempt (2).
	// 3) Execute the connection attempt inside a goroutine: `go func() { ... }()`
	// 	- Why goroutine?
	// 	- Because if we executed `DialContext` in the same thread, the program would get stuck and, we couldn't call `cancel()` at the same time.
	// 	- Inside it, we put a `defer`: `defer func() { sync <- struct{}}{} }()`, Simple meaning:
	// 	- Whatever happens (error, return, ...), it finally sends a message to the sync channel so that we know "it's done".
	go func() { //(2)
		defer func() { sync <- struct{}{} }()
		// 4) Create a Dialer, and intentionally slow down the connection:
		// 	- First create a Dialer: `var d net.Dialer`
		//	- Next: `d.Control = func(...) error { time.Sleep(time.Second); return nil }`, Simple meaning:
		// 		- With Control we intentionally delay 1 second
		// 		- Why?
		// 			- So that the connection remains “trying” and we have the opportunity to quickly call `cancel()`
		var d net.Dialer
		d.Control = func(_, _ string, _ syscall.RawConn) error {
			time.Sleep(time.Second)
			return nil
		}
		// 5) Attempt to connect with DialContext: `conn, err := d.DialContext(ctx, "tcp", "10.0.0.1:80")`, Simple meaning:
		// 	- Go and connect using ctx
		// 	- Note: Because `ctx` is cancelable, if it is canceled:
		// 		- `DialContext` should stop immediately and throw an `err`
		conn, err := d.DialContext(ctx, "tcp", "10.0.0.1:80")
		// 6) Check the result inside the goroutine
		// 	- `if err != nil { t.Log(err); return }`, Simple meaning:
		// 		- If we get an error: Log it and Return
		// 			- In this test, we expect this error because we are going to cancel.
		// 		- If there was no error:
		// 			- That means the connection was established!, Then: `_ = conn.Close()`
		//			- and `t.Error("connection was not canceled")`, the meaning is:
		// 				- It should not have been connected, it should have been disconnected
		if err != nil {
			t.Log(err)
			return
		}
		_ = conn.Close()
		t.Error("connection was not canceled")
	}()
	// Once the dialer is attempting to connect to and handshake with the remote node, you call the cancel function (3) to cancel the context.
	// What happens outside the goroutine?
	// 	- 7) Canceling the context, Simple meaning:
	// 		- We press the cancel button
	// 		- It tells all operations that have ctx: “Stop!”
	cancel() // (3)
	// 	- 8) Wait for the goroutine to finish: `<-sync`, Simple meaning:
	// 		- We wait for the goroutine to signal that it has finished
	// 		- This will cause the test:
	// 			- not to finish before the checks are complete
	<-sync
	// You can check the context’s `Err()` method to make sure the call to cancel was what resulted in the canceled context,
	// as opposed to a deadline in Listing 3-6. In this case, the context’s `Err()` method should return a `context.Canceled` error (4).
	// 9) Final check: What was the reason for cancellation?
	// 	- `if ctx.Err() != context.Canceled { ... }`, Simple meaning:
	// 		- We make sure the context was canceled due to manual cancel
	// 		- If it was a deadline, it would be `DeadlineExceeded`
	// 		- But here it should be: `context.Canceled`
	if ctx.Err() != context.Canceled { // (4)
		t.Errorf("expected canceled contex; actual: %q", ctx.Err())

	}

}
