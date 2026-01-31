package ch03

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"
)

// --- STEP 7 ---
// ## Canceling Multiple Dialers
// - You can pass the same context to multiple `DialContext` calls and cancel all the calls at the same time by executing the context’s cancel function.
//   - For example, let’s assume you need to retrieve a resource via TCP that is on several servers.
//   - You can asynchronously dial each server, passing each dialer the same context.
//   - You can then abort the remaining dialers after you receive a response from one of the servers.
//   - In Listing 3-8, you pass the same context to multiple dialers.
//   - When you receive the first response, you cancel the context and abort the remaining dialers.
//
// - You create a context by using context.WithDeadline (1)
//   - because you want to have three potential results when checking the context’s Err method:
//   - `context.Canceled`, `context.DeadlineExceeded`, or nil
//   - You expect Err will return the `context.Canceled `error,
//   - since your test aborts the dialers with a call to the cancel function.
//
// - First, you need a listener. This listener accepts a single connection and closes it after the successful handshake (2)
// - Next, you create your dialers. Since you’re spinning up multiple dialers,
//   - it makes sense to abstract the dialing code to its own function (3)
//   - This anonymous function dials out to the given address by using DialContext.
//   - If it succeeds, it sends the dialer’s ID across the response channel, provided you haven’t yet canceled the context.
//   - You spin up multiple dialers by calling dial in separate goroutines using a for loop (4)
//
// - If dial blocks on the call to `DialContext` because another dialer won the race,
//   - you cancel the context, either by way of the cancel function or the deadline, causing the dial function to exit early.
//
// - You use a wait group to make sure the test doesn’t proceed until all dial goroutines terminate after you cancel the context.
// - Once the goroutines are running, one will win the race and make a successful connection to the listener.
//   - You receive the winning dialer’s ID on the `res` channel (5), then abort the losing dialers by canceling the context.
//   - At this point, the call to wg.Wait blocks until the aborted dialer goroutines return.
//
// - Finally, you make sure it was your call to cancel that caused the cancelation of the context (6).
//   - Calling cancel does not guarantee that Err will return `context.Canceled`.
//   - The deadline can cancel the context, at which point calls to cancel become a no-op and Err will return context.DeadlineExceeded.
//   - In  practice, the distinction may not matter to you, but it’s there if you need it.
//
// ---
// The big picture
//   - Suppose you want to get a file/resource from multiple possible servers:
//   - You send 10 people to knock on the door at the same time
//   - You accept whoever answers first
//   - As soon as one person answers → you tell the others to go back! (cancel)
//   - Here “go back!” is the same as cancel() and this message is sent to everyone via ctx.
//
// - Fan-out: “Dial” multiple servers at the same time
// - Fan-in / Cancel: As soon as one answers, disconnect the others with the same context
//
// Listing 3-8: Canceling all outstanding dialers after receiving the first response
func TestDialContextCancelFanOut(t *testing.T) {
	// Test 1: "with at least one answer"
	// 	- This scenario says: at least one server will respond (so the context should end with Canceled)
	t.Run("with at least one answer", func(t *testing.T) {
		// Step 1) Create a context with a deadline (10 seconds)
		// 	- `ctx` is a “common tab” between all dialers
		// 	- It has a 10-second deadline because it wants to have 3 possible states:
		// 		- `nil` (nothing has happened yet)
		// 		- `context.Canceled` (we canceled it ourselves)
		// 		- `context.DeadlineExceeded `(time expired)
		// 		- `defer cancel()` is for cleanup (if it finishes earlier, resources will be freed)
		ctx, cancel := context.WithDeadline( // (1)
			context.Background(),
			time.Now().Add(10*time.Second),
		)
		defer cancel()

		// Step 2) Create a small local server (listener)
		// This will create a "test server" on your computer
		// 	- "127.0.0.1:" means:
		// 		- Listen on local
		// 		- Choose the port yourself (that's why : is empty)
		listener, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Fatal(err)
		}
		defer listener.Close()

		// Step 3) Make this server accept only one connection
		// 	- Accept() means wait for someone to connect
		// 	- As soon as someone connects:
		// 		- Closes the connection
		// 		- So the result:
		// 			- Only one can "connect successfully"
		// 			- The rest either fail or arrive late and are disconnected with cancel
		go func() { //(2)
			// Only accepting a single connection.
			conn, err := listener.Accept()
			if err == nil {
				_ = conn.Close()
			}
		}()
		// This goroutine goes into wait mode on: `listener.Accept()`
		// 	- It accepts the first connection it comes across and gets the `conn`.
		// 	- Then if there are no errors:
		// 		- It immediately Closes() the same conn.
		// 		- And then the goroutine terminates.
		// 	- So the practical result:
		// 		- This goroutine only accepts once (because it is not in a loop)
		// 		- After that, it does not accept any other connections
		// 		- So in this test, only one dialer has a real chance of “successfully connecting” (the one that arrived first)
		// - But one important point:
		// 		- The listener itself is still open (until the defer listener.Close() is executed)
		//		- That means the other dialers may:
		// 			- either get stuck/timeout
		// 			- or get an error
		//			- or get interrupted in the middle because the context is canceled
		//			- (depending on the system's scheduling)
		// - But from the perspective of the “one who accepts”:
		//		- Only that one connection is processed and then we don't receive anymore.
		// ---

		// Step 4) Define the dial function (common code for each dialer)
		// What does this function do?
		// 	- It tries to connect to the address:
		//		- With DialContext
		// 		- That is, if ctx is canceled, this attempt should be aborted
		// 	- If the connection was successful:
		// 		- It closes the connection (because it is just a test)
		// 		- Then it wants to say "I succeeded" and send the id in the response
		// 	- But one very important point:
		// 		- This select says:
		// 			- If the context was canceled by then → do not send anything anymore
		// 			- If it was not canceled → send the id
		// 		- This makes:
		// 			- After one person wins and, we cancel,
		// 			- the rest are not stuck trying to send anything in the channel.
		dial := func(ctx context.Context, address string, response chan int, id int) { // (3)
			var d net.Dialer
			c, err := d.DialContext(ctx, "tcp", address)
			if err != nil {
				return
			}
			_ = c.Close()

			select {
			case <-ctx.Done():
			case response <- id:
			}
		}
		// Step 5) Create the response channel and waitgroup
		//	- res is the channel where the “first successful dialer” sends its number
		// 	- wg is for waiting until all goroutines are finished at the end of the test
		res := make(chan int)
		var wg sync.WaitGroup

		// Step 6) Create 10 simultaneous dialers (Fan-out)
		//	- Here 10 goroutines are created that call simultaneously
		// 	- Each one has a different id (1 to 10)
		// Very important point (conceptual understanding):
		// 	- The goal is to create a “competition”:
		// 	- Whoever connects first wins
		for i := 0; i < 10; i++ { // (4)
			wg.Go(func() { dial(ctx, listener.Addr().String(), res, i+1) })
		}
		// Note 1) How is the "anonymous function" implemented here?
		// 	- This part: `func() { dial(...) }`
		// 		- Creates an anonymous function (i.e. a function that has no name).
		// 		- Important point:
		// 			- Since it doesn't end with (), you didn't execute it yourself.
		// 			- You just "delivered" this function to `wg.Go`.
		// 			- So the actual execution of this function is done by wg.Go, not you.
		// 		- That is, wg.Go behaves like this:
		// 			- "Run this function inside a goroutine"
		// 			- Result: `func(){...}` here plays the role of "the work to be done" (Job).
		// Note 2) What does this goroutine + WaitGroup model mean and why is it used?
		//	- The purpose of WaitGroup is to:
		// 		- When you have multiple goroutines running,
		// 		- the program/test will wait until they all finish.
		// 		- In standard Go form, it usually looks like this:
		//		```wg.Add(1)
		//			go func() {
		//    			defer wg.Done()
		//    			dial(...)
		//   			}()```
		//			- This means is:
		// 				- `Add(1)` → says "A task has started"
		// 				- `go func(){...}` → runs the task simultaneously
		// 				- `Done()` → says "This task is done"
		// 				- Wait() → waits until the number of remaining tasks reaches zero
		//		- Now wg.Go(func(){...}) (if it really exists) is just a shortcut that does the same things all at once:
		// 			- It does Add(1)
		// 			- It runs the goroutine
		//			- It finally calls `Done()`
		// 			- Result: You just give it the "work", and wg handles the counting and waiting.
		// ---

		// Step 7) We wait for either someone to respond or the deadline to arrive.
		// 	- If the deadline is reached before the response → ctx.Done() is triggered
		// 	- If someone sends the id:
		// 		- the response becomes that id
		// 		- Then we quickly cancel() to stop all other dialers
		// 	- This is the most important part of the first test.
		var response int
		select {
		case <-ctx.Done():
		case response = <-res: // (5)
			cancel()
		}

		// Step 8) We wait for all dialers to finish.
		//	- That is, until all goroutines return.
		wg.Wait()
		// Why do we still need `wg.Wait()` even though the loop is finished?
		// 	- Inside the loop, you do this:
		// 		- Start 10 goroutines
		// 		- But the goroutines: Run asynchronously
		// 		- They may still be `DialingContext`
		//		- Or waiting for the network/Context/Channel
		// 		- So when the loop finishes, it means:
		// 			- "All goroutines have started"
		// 			- Not: "All goroutines have finished"
		// What exactly does `wg.Wait()` do?
		// 	- wg.Wait() says:
		// 		- "Wait until all goroutines that wg has counted have called Done()."
		// 		- That is:
		// 			- The program will not move beyond this line until they have all finished.
		// What is the use of this here?
		// 	- In this test, there are several very important reasons:
		// 		- 1) Prevent premature termination of the test
		// 			- If `wg.Wait()` is not there:
		// 				- The test may finish
		// 				- But some goroutines are still running
		// 				- Result: Tests will be flaky or incomplete
		// 		- 2) Prevent goroutine leaks
		// 			- Some goroutines may still be stuck (e.g. waiting for a dial or channel)
		// 			- `wg.Wait()` helps to make sure that they are all collected.
		// 		- 3) Check the final result correctly
		// 			- After select and cancel, you check what happened to `ctx.Err()`
		// 			- But if some goroutines are still running, the behaviors/logs may still be ongoing.
		// ---

		// Step 9) Checks if the reason for ending the context was cancel
		//	- Because "We expect" is found to be the winner and cancel() is called
		// 	- So `ctx.Err()` should be `context.Canceled`
		if !errors.Is(ctx.Err(), context.Canceled) { // (6)
			t.Errorf("expected canceled context; actual: %s",
				ctx.Err(),
			)
		}

		// Step 10) Print which dialer won
		t.Logf("dialer %d retrieved the resource", response)
	})

	// Test 2: "without an answer"
	// This scenario says: no server is responding (so it must be due).
	t.Run("without an answer", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(
			context.Background(),
			time.Now().Add(10*time.Second),
		)
		defer cancel()

		// The main difference between this test and the previous one is:
		//	- It creates a listener but closes it immediately
		// 	- meaning no one can connect
		// 	- so all dialers try and fail
		listener, err := net.Listen("tcp", "127.0.0.1:")
		if err != nil {
			t.Fatal(err)
		}
		// close the listener immediately to prevent a connection
		_ = listener.Close()

		dial := func(ctx context.Context, address string, response chan int, id int) {
			var d net.Dialer
			c, err := d.DialContext(ctx, "tcp", address)
			if err != nil {
				return
			}
			c.Close()

			select {
			case <-ctx.Done():
			case response <- id:
			}
		}

		res := make(chan int)
		var wg sync.WaitGroup

		// Fan-out again the same 10 dialers
		//	- The same loop repeated.
		for i := 0; i < 10; i++ { // (4)
			wg.Go(func() { dial(ctx, listener.Addr().String(), res, i+1) })
		}
		// 	- What happens to this select?
		//		- Since no dialer succeeds, no one sends anything to `res`
		// 		- So the only way to exit select is if:
		// 		- deadline is reached → ctx.Done() is triggered
		var response int
		select {
		case <-ctx.Done():
		case response = <-res:
			cancel()
		}

		wg.Wait()
		// Then it checks what Err is.
		// 	- Because the time must have expired
		// 	- So it must be DeadlineExceeded
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			t.Errorf("expected deadline exceeded; actual: %s",
				ctx.Err(),
			)
		}
		// And she checks and doesn't get any real answers.
		// 	- If response never receives anything from the channel, it will remain the same initial value: 0
		// 	- So if it becomes non-zero, it means that the dialer somehow succeeded (which it shouldn't).
		if response != 0 {
			t.Fatalf("expected a response of 0; actual: %d", response)
		}

		t.Log("no dialer retrieved the resource")
	})
}
