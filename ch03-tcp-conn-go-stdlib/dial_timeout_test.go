package ch03

import (
	"net"
	"syscall"
	"testing"
	"time"
)

// --- STEP 4 ---

// ## Timing Out a Connection Attempt with the DialTimeout Function
// Using the Dial function has one potential problem:
// 	- you are at the mercy of the operating system to time out each connection attempt.
//	- For example, if you use the Dial function in an interactive application and your operating system times out connection attempts after two hours,
//	- your application’s user may not want to wait that long, much less give your app a five-star rating.
// To keep your applications predictable and your users happy, it’d be better to control time-outs yourself.
//	- For example, you may want to initiate a connection to a low-latency service that responds quickly if it’s available.
//	- If the service isn’t responding, you’ll want to time out quickly and move onto the next service.
// One solution is to explicitly define a per-connection time-out duration and use the DialTimeout function instead. Listing 3-5 implements this solution.

// Listing 3-5: Specifying a time-out duration when initiating a TCP connection
//   - Since the net.DialTimeout function (1) does not give you control of its net.Dialer to mock the dialer’s output,
//     you’re using our own implementation that matches the signature.
//   - Your DialTimeout function overrides the Control function (2) of the net.Dialer to return an error. You’re mocking a DNS time-out error.
// ---
// This DialTimeout is basically a training/test trick to intentionally mock a "timeout" error
// 	without actually connecting to the network and see how our program handles it.
// 	1) Why write this function at all?
// 		- `net.DialTimeout(...)` creates a `net.Dialer` inside itself, and you don't have direct control over the Dialer to "fake" its output in the test.
//	 	- So the author wrote a new function called `DialTimeout`, whose signature is exactly the same as `net.DialTimeout`:
//	 		- `func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error)`
//	 		- This is called "matching the signature", meaning:
//	 			- You can use this instead of `net.DialTimeout` in your test code, because its input/output is the same.

func DialTimeout(network, address string, timeout time.Duration) (net.Conn, error) { // (1)
	// 2) What did he do inside?
	// 		- He made a Dialer:
	//			- Timeout: timeout means: Wait this long before dialing.
	// 3) What does Control mean?
	// 		- Control is a “hook” that Go calls before the TCP connection is actually established.
	// 		- In the real world, it is used for low-level socket configuration (like socket options).
	// 		- But here the author has put it to good use 😄:
	// 			- He said: “Whenever you try to connect, return an error first!”
	d := net.Dialer{
		Control: func(_, addr string, _ syscall.RawConn) error { // (2)
			// 4) What is this error?
			//	- This means it creates a DNS error of type `net.DNSError` which:
			// 		- IsTimeout: true → means this error is timeout
			// 		- IsTemporary: true → means it is temporary and can be retried
			// 		- This is very important because `net.DNSError` supports behaviors like `Timeout()` and `Temporary()`, so later you can write:
			// 			- `if nErr, ok := err.(net.Error); ok && nErr.Timeout() { ... }`
			return &net.DNSError{
				Err:         "connection timed out",
				Name:        addr,
				Server:      "127.0.0.1",
				IsTimeout:   true,
				IsTemporary: true,
			}
		},
		Timeout: timeout,
	}
	// Finally, this line is executed:
	// 	- But because Control gives an error right away, the connection process is interrupted and Dial returns the same error.
	//  - So in practice, this function:
	// 		- Does not always (or in this case) establish a connection
	// 		- And only returns an artificial `timeout error`.
	return d.Dial(network, address)
}

// Unlike the `net.Dial` function, the DialTimeout function includes an additional argument, the time-out duration (3).
// Since the time-out duration is five seconds in this case, the connection attempt will time out if a connection isn’t successful within five seconds.
// In this test, you dial 10.0.0.0, which is a non-routable IP address, meaning your connection attempt assuredly times out.
// For the test to pass, you need to first use a type assertion to verify you’ve received a `net.Error `(4) before you can check its Timeout method (5).
//
// If you dial a hostname that resolves to multiple IP addresses(such as IPv4 and IPv6 or multiple servers),
// 	- Go starts a connection race between each IP address, giving the primary IP address a head start.
//	- The first connection to succeed persists, and the remaining contenders cancel their connection attempts.
// 	- If all connections fail or time out, `net.DialTimeout` returns an error.

func TestDialTimeout(t *testing.T) {
	// What is the purpose of the test?
	// 	- It says:
	// 		- I want to connect, but if I don’t connect within 5 seconds, it should `Timeout`.
	// 		- If it doesn’t `Timeout`, then my code is not correct.”
	c, err := DialTimeout("tcp", "10.0.0.1:http", 5*time.Second) // (3)
	// If we had no errors, it means that the timeout did not occur → the test should fail.
	if err == nil {
		_ = c.Close()
		t.Fatal("connection did not time out")
	}
	// Now we need to make sure the error is of type `net.Error`, because only `net.Error` has the `Timeout()` method.
	// `ok == true`, means this error has `Timeout()` capability
	// If `ok == false`, means this error is not a standard network error and, we cannot check if it is Timeout → test fail
	nErr, ok := err.(net.Error) // (4)
	if !ok {
		t.Fatal(err)
	}
	// Let's check if it really timed out or not.
	// If `Timeout()` is false, it means:
	// 	- The connection failed, but not because of a timeout (e.g. “connection refused” or something else)
	// 	- Then the test fails.
	if !nErr.Timeout() { // (5)
		t.Fatal("error is not a timeout")
	}
}

// ## `net.Dial` and `net.Dialer` are both for "connecting", but one is a ready-made function and one is a configurable tool.
//
// ### What is `net.Dial`?
// 	- A simple function for when you don't want any special settings:
// 		- `conn, err := net.Dial("tcp", "example.com:80")`
// 	- Fast and easy
// 	- The settings (timeouts, local address, keepalive, Control, etc.) are up to Go/OS or defaults.
//
// ### What is `net.Dialer`?
// 	- A struct (configuration) that allows you to specify exactly how to make a connection, then dial it:
// 		- `d := net.Dialer{Timeout: 2 * time.Second}`
// 		- `conn, err := d.Dial("tcp", "example.com:80")`
// 	- With Dialer you can control things like:
// 		- Timeout (how long to wait for a connection)
// 		- Deadline or KeepAlive
// 		- Choose local address (which IP/port to connect from)
// 		- Control for low-level socket settings
//			- What does this “Control:” mean?
//				- `Control` field is look like this:
// 					- `Control func(network, address string, c syscall.RawConn) error`
// 					- That is: Control is a “function” field.
// 					- So when you write:
// 						- `d := net.Dialer{ Control: func(...) error { ... },}`
//						- You are saying: For my Dialer, set this function as Control.
//			- 2) What exactly does Control do?
// 				- When d.Dial(...) is executed and the OS is creating the socket,
//	          	  Go calls this function before the actual connection is complete, if Control is set.
// 				- The main purpose of Control in the real world:
//					- Lets you do low-level socket setup before connecting, for example:
// 						- Setting SO_REUSEADDR
// 						- Setting keepalive
// 						- Binding to a specific interface
//						- Specific network setup
// 					- These are “OS” tasks.
// 	- Why did you put _ in the inputs?
// 		- Here: func(_, addr string, _ syscall.RawConn) error
// 		- The _ symbol means:
// 		- This input exists, but I don’t use it in the function body.
// 		- In fact, the author says:
// 			- I don’t want the first parameter (like `network`)
//			- I don’t want the third parameter (`RawConn`) either
// 			- Only `addr` is important to me
// 		- So _ is put so that Go doesn’t get caught saying “you declared a variable but didn’t use it”.
