package ch03

import (
	"context"
	"net"
	"syscall"
	"testing"
	"time"
)

// --- STEP 5 ---
// ## Using a Context with a Deadline to Time Out a Connection
// A more contemporary solution to timing out a connection attempt is to use a context from the standard library’s context package.
// 	- A context is an object that you can use to send cancellation signals to your asynchronous processes.
//	- It also allows you to send a cancellation signal after it reaches a deadline or after its timer expires.
// 	- All cancellable contexts have a corresponding `cancel()` function returned upon instantiation.
//		- The `cancel()` function offers increased flexibility since you can optionally cancel the context before the context reaches its deadline.
//		- You could also pass along its `cancel()` function to hand off cancellation control to other bits of your code.
//			- For example, you could monitor for specific signals from your operating system,
//		 	  such as the one sent to your application when a user presses the ctrl-C key combination,
//	          to gracefully abort connection attempts and tear down existing connections before terminating your application.
// - Listing 3-6 illustrates a test that accomplishes the same functionality as DialTimeout, using context instead.

// Listing 3-6: Using a context with a deadline to time out the connection attempt
func TestDialContex(t *testing.T) {
	// Before you make a connection attempt, you create the context with a deadline of five seconds into the future (1),
	// after which the context will automatically cancel.
	dl := time.Now().Add(5 * time.Second) // (1)
	// Next, you create the context and its cancel function by using the `context.WithDeadline()` function (2),
	// 	- setting the deadline in the process.
	//	- It’s good practice to defer the cancel function (3) to make sure the context is garbage collected as soon as possible.
	ctx, cancel := context.WithDeadline(context.Background(), dl) // (2)
	defer cancel()                                                // (3)

	// Then, you override the dialer’s Control function (4) to delay the connection just long enough to make sure you exceed the context’s deadline.
	var d net.Dialer // DialContext is a method on a Dialer
	// What exactly is `d.Control`?
	// 	- `Control` is a callback (hook function) on `net.Dialer` that Go calls during socket/connection creation.
	// 	- This means that before/while the connection is actually established, this function is executed so that you can do things like:
	// 		- Low-level socket settings (like `setsockopt`)
	// 		- Or in this test: artificial delays.
	d.Control = func(_, _ string, _ syscall.RawConn) error { // (4)
		// Sleep long enough to reach the context's deadline.
		// Why did you put `+ time.Millisecond`?
		// 	- Because if you put exactly 5*time.Second:
		// 		- It could be right on the border due to system timing/accuracy of timers and the test would sometimes be flaky
		// 		- With +1ms they make sure they will definitely miss the deadline
		time.Sleep(5*time.Second + time.Millisecond)
		return nil
		// So what does this code do?
		// 	- You override Control and put: `time.Sleep(5*time.Second + time.Millisecond)`
		// 	- This means:
		// 		- Hold it for more than 5 seconds
		// 		- And since the deadline context is set exactly 5 seconds ahead:
		// 			- When DialContext(ctx, ...) is waiting for the connection result
		// 			- the deadline is reached and ctx is canceled
		//			- Then Dial should return with an error
		// 		- More precisely:
		//			- Dial starts But you intentionally slow it down with Control
		// 			- And the result is that it timeouts “before the connection is completed”
	}

	// Finally, you pass in the context as the first argument to the `DialContext()` function (5).
	// The sanity check (6) at the end of the test makes sure that reaching the deadline canceled the context, not an erroneous call to cancel.
	//
	// As with DialTimeout, if a host resolves to multiple IP addresses, Go starts a connection race between each IP address, giving the primary IP address a head start.
	//	- The first connection to succeed persists, and the remaining contenders cancel their connection attempts.
	//	- If all connections fail or the context reaches its deadline, `net.Dialer.DialContext` returns an error.
	//
	// This is the main line: Attempting to connect with Context
	// 	- `conn, err := d.DialContext(ctx, "tcp", "10.0.0.0:80")` That is:
	// 		- With this dialer (which Control intentionally slowed down), go to address 10.0.0.0 on port 80
	// 		- But because ctx deadline = 5 seconds and, we slept for more than 5 seconds inside Control:
	// 		- The deadline must be reached before the connection can be established and the connection will time out
	conn, err := d.DialContext(ctx, "tcp", "10.0.0.0:80") // (5)
	// If err == nil, it means the connection was successful and no timeout occurred → it should fail.
	if err == nil {
		_ = conn.Close()
		t.Fatal("connection did not time out")
	}
	// Why does it check `net.Error`?
	//	- `nErr, ok := err.(net.Error)` means:
	// 		- Is this err a network error that has more information?
	// 		- If it is:
	// 			- It checks `nErr.Timeout()` means:
	// 				- Is this error really a timeout?
	// 				- This is very important because:
	// 					- The error may not be a "timeout", for example:
	//						- network unreachable
	// 						- connection refused
	//						- or anything else
	// - This test wants to see exactly the timeout
	nErr, ok := err.(net.Error)
	if !ok {
		t.Error(err)
	} else {
		if !nErr.Timeout() {
			t.Errorf("error is not a timeout: %v", err)
		}
	}
	// Final check: Make sure the cancellation was due to Deadline
	// 	- if `ctx.Err() != context.DeadlineExceeded { ... }` means:
	// 		- What was the reason for the context cancellation?
	// 	- If the deadline has been reached:
	// 		- `ctx.Err()` becomes `context.DeadlineExceeded`
	//  - If someone manually called cancel():
	// 		- `ctx.Err()` usually becomes `context.Canceled`
	// - So this line ensures:
	// 		- The cancellation was due to timeout, not manual cancellation
	if ctx.Err() != context.DeadlineExceeded { // (6)
		t.Errorf("expected deadline exceeded; actual: %v", ctx.Err())
	}
}

// What exactly is ctx? (its type/kind)
// 	- ctx is an object/value of type `context.Context` (actually an interface in Go)
// 	- What does it mean?
// 		- Context is a "contract" that guarantees a few standard features:
// 			- `Done()`: Provides a channel that receives a packet/signal when the operation needs to stop
// 			- `Err()`: Tells why it was canceled (e.g. deadline reached or manually canceled)
// 			- `Deadline()`: Returns the time if there is a deadline
// 			- `Value(key)`: Used to carry very lightweight information down the call chain (e.g. request-id)
// What does `context.WithDeadline(...)` return?
// 	- ctx returns:
// 		- A new context that: Has a specified deadline
// 		- When the deadline is reached:
// 			- ctx.Done() is called
// 			- ctx.Err() is usually context.DeadlineExceeded
// 	- cancel returns:
// 		- A function of type `context.CancelFunc` (roughly: func())
// 		- When called: The same thing happens as cancel, even if you haven't reached the deadline yet
// What is context.Background() and why did we put it in?
// 	- `context.Background()` returns a base/root context
// 	- Its features:
// 		- No deadline
// 		- No cancel
// 		- No special value
// 	- Why is it used?
// 		- Because `WithDeadline` must be created on a parent context:
// 			- That is, it says: "Create a new context that is a child of this parent"
// 	- Here you put the parent in `Background()` because:
// 		- In simple test/code, you don't have ctx from somewhere higher
// 		- You start from the root
// 	- A very simple example to understand the hierarchy (Parent/Child)
// 		- Imagine the contexts are like a tree:
// 			- `Background()` = root
// 		- `WithDeadline(Background(), ...)` = a new branch
// 		- If the parent is canceled:
// 			- The children are also canceled
// 		- But if the child is canceled:
// 			- The parent is not affected
// Why is `defer cancel()` a “best practice”?
// 	- Even if deadline cancels itself after 5 seconds:
// 		- You may exit the function early (e.g., if the test fails or, you return early)
// 	- defer cancel() helps:
// 		- Timers/internal context resources are freed faster
// 		- Goroutines that are waiting for ctx will hang around for no reason.
