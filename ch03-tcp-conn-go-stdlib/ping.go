package ch03

import (
	"context"
	"io"
	"time"
)

// --- STEP 8 ---
// ## Implementing a Heartbeat
// For long-running network connections that may experience extended idle periods at the application level,
// it’s wise to implement a heartbeat between nodes to advance the deadline.
//	- This allows you to quickly identify network issues and promptly reestablish a connection as opposed
// 	  to waiting to detect the network error when your application goes to transmit data.
//	- In this way, you can help make sure your application always has a good network connection when it needs it.
// For our purposes, a heartbeat is a message sent to the remote side with the intention of eliciting a reply,
// which we can use to advance the deadline of our network connection.
// 	- Nodes send these messages at a regular interval, like a heartbeat.
//	- Not only is this method portable over various operating systems,
//    but it also makes sure the application using the network connection is responding, since the application implements the heartbeat.
//  - Also, this technique tends to play well with firewalls that may block TCP keepalives. We’ll discuss keepalives in Chapter 4.
// To start, you’ll need a bit of code you can run in a goroutine to ping at regular intervals.
//	- You don’t want to needlessly ping the remote node when you recently received data from it,
//	- so you need a way to reset the ping timer.
//	- Listing 3-10 is a simple implementation from a file named ping.go that meets those requirements.
// I use ping and pong messages in my heartbeat examples, where the reception of a ping message—the challenge—tells
//	- the receiver it should reply with a pong message—the response.
//	- The challenge and response messages are arbitrary.
//	- You could use anything you want to here, provided the remote node knows your intention is to elicit its reply.

// Listing 3-10: A function that pings a network connection at a regular interval
// This Pinger function creates a "heartbeat" that:
// 	- sends a "ping" message on a connection/output (`w`) once every second interval
// 	- but if necessary, you can reset/change the interval (via the reset channel)
// 	- and it stops the whole thing cleanly whenever `ctx` is canceled
// Inputs Role:
// 	- `ctx context.Context`
// 		- Like “off-key”, Whenever canceled → pinger should stop
// 	- `w io.Writer`
// 		- Where ping is written
// 		- Can be `net.Conn` (since it has Written), or anything else
// 	- `reset <-chan time.Duration`
// 		- A channel from which you can tell pinger:
// 			- “Reset the timer” or “This is the new interval”
//
// 	- The Pinger function writes ping messages to a given writer at regular intervals.
//		- Because it’s meant to run in a goroutine, Pinger accepts a `context` as its first argument so you can terminate it and prevent it from leaking.
//		- Its remaining arguments include an `io.Writer` interface and a channel to signal a timer reset.
//		- You create a buffered channel and put a duration on it to set the timer’s initial interval (1).
//		- If the interval isn’t greater than zero, you use the default ping interval.
// 	- You initialize the timer to the interval (2) and set up a deferred call to drain the timer’s channel to avoid leaking it, if necessary.
// 	- The endless for loop contains a select statement, where you block until one of three things happens:
//		- the context is canceled, a signal to reset the timer is received, or the timer expires.
//			- If the context is canceled (3), the function returns, and no further pings will be sent.
//			- If the code selects the reset channel (4), you shouldn’t send a ping, and the timer resets (6) before iterating on the select statement again.
// 			- If the timer expires (5), you write a ping message to the writer, and the timer resets before the next iteration.
//	- If you wanted, you could use this case to keep track of any consecutive time-outs that occur while writing to the writer.
//		- To do this, you could pass in the context’s cancel function and call it here if you reach a threshold of consecutive time-outs.

// ---
// Step 0) Default value
//   - If interval is not specified, a ping is performed every 30 seconds.
const defaultPingInterval = 30 * time.Second

func Pinger(ctx context.Context, w io.Writer, reset <-chan time.Duration) {

	// Step 1) Get the initial interval
	// 	- This section has three states:
	// 		1. If ctx was canceled from the beginning → return quickly
	// 		2. If you have sent a value to reset from outside → it takes the interval from it
	// 		3. If there was nothing on reset → default: it executes and does not wait
	//
	// 	- Important point:
	// 		- default prevents this select from blocking
	// 		- That is, the function does not get stuck, which must definitely take a value from reset.
	var interval time.Duration
	select {
	case <-ctx.Done():
		return
	case interval = <-reset: // (1) pulled initial interval off reset channel
	default:
	}
	// Step 2) If interval was bad, default
	// 	- If interval is zero or negative → it will set it to 30 seconds(Default Value).
	if interval <= 0 {
		interval = defaultPingInterval
	}

	// Step 3) Making the timer
	//	- Creates a timer that sends a signal to `timer.C` after a specified interval.
	timer := time.NewTimer(interval) // (2)
	// Step 4) Timer cleaning with defer
	// 	- This means:
	//		- When the function finishes, stop the timer
	// 		- If stopping “fails” it means:
	// 			- The timer may have “fired” at the same time and there is something left in timer.C
	// 			- So it reads timer.C once to empty the channel
	// 		- Purpose:
	// 			- Preventing timers from getting stuck/leaking resources/behaving strangely
	defer func() {
		if !timer.Stop() {
			<-timer.C
		}
	}()

	// Step 5) Main loop (pinger works constantly)
	// Each time the circle goes around, one of the following happens:
	//	- Case A) ctx canceled
	// 	- Case B) A new interval came from the reset channel. means:
	// 		- Someone from outside said: “Change/reset interval”
	// 		- First it stops the previous timer and if necessary it drains its channel
	// 		- Then if the new value was positive:
	//				- it changes the interval with it
	// 		- If the new value was zero/negative:
	//			- it does not change the interval (it remains the same)
	// 		- Result: The timer is reset and the count starts again
	// 	- Case C) Timer rang → It's time to ping. means:
	//		- interval ended
	//		- The pinger writes "ping" to w
	// 		- If the write fails:
	// 			- This means there is probably a connection problem → the function returns (stops)
	// 			- The comment says that here you can count the number of consecutive timeouts and make a more serious decision (e.g. reconnect).
	for {
		select {
		case <-ctx.Done(): // (3)
			return
		case newInterval := <-reset: // (4)
			if !timer.Stop() {
				<-timer.C
			}
			if newInterval > 0 {
				interval = newInterval
			}
		case <-timer.C: // (5)
			if _, err := w.Write([]byte("ping")); err != nil {
				// track and act on consecutive timeouts here

				return
			}
		}
		// Step 6) Reset the timer for the next round. means:
		// 	- After one of those conditions occurs, the timer is reset for the next interval
		// 	- So the cycle is:
		// 	 	- Wait → or reset → or ping → wait again → …
		_ = timer.Reset(interval) // (6)
	}
}

// 1) What does `time.NewTimer` return?
// 	- Returns a value of type `*time.Timer` (i.e. a "timer object").
// 2) Where did `C` come from?
// 	- The `time.Timer` type in the time package has a field called `C`. That is, its structure is roughly as follows:
// 		- timer is a struct
// 		- `C` is one of its fields
// 	- In simple terms:
// 		- `timer.C` means: "The channel through which the timer announces that the time has expired"
// 	- 3) What exactly is `timer.C`?
// 		- `timer.C` is of type:
// 			- `<-chan time.Time`, It means:
// 				- A read-only channel
//				- that sends a time value (time.Time) into it when the timer "rings".
// 4) So why do they just write `<-timer.C` in the code and not get its value?
// 	- Because here they just want to “wait for the timer to ring”:
// 		- `case <-timer.C`:
//			- Timer ended
// 			- We don’t need the time itself that comes into the channel
// 			- Just the fact that “it happened” is enough
// 	- If you want to get its time as well, you can:
//		```case t := <-timer.C:
//    			fmt.Println("timer fired at", t)```
// 5) A very simple analogy
// 	- A timer is like an alarm clock
// 	- `timer.C` is like a “bell/wire” that when the clock rings, a signal comes
// 	- `<-timer.C` means:
// 		- “Wait for the bell to ring”
// 6) Difference with time.After
// 	- You may have seen:
// 		- `<-time.After(5 * time.Second)`
// 		- `time.After` also actually returns a channel
// 	- `Timer` is a “more controllable” version of it (you can Stop/Reset)
