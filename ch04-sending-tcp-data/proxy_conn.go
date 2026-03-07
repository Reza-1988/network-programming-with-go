package ch04

import (
	"io"
	"net"
)

// ## Proxying Data Between Connections
// Listing 4-14: Proxying data between two network connections
// 	- The `io.Copy` function does all the heavy input/output (I/O) lifting for you.
//		- It takes an `io.Writer` as its first argument and an `io.Reader` as its second argument.
//		- It then writes, to the writer, everything it reads from the reader until the reader returns an `io.EOF`, or,
//		- alternately, either the reader or writer returns an error.
//		- The `io.Copy `function returns an error only if a `non-io.EOF` error occurred during the copy,
//		- because `io.EOF` means it has read all the data from the reader.
// 	- You start by creating a connection to the source node (1) and a connection to the destination node (2).
//		- Next, you run `io.Copy` in a goroutine, reading from `connDestination` and writing to `connSource` (3) to handle any replies.
//		- You don’t need to worry about leaking this goroutine, since `io.Copy` will return when either connection is closed.
//		- Then, you make another call to  `io.Copy`, reading from `connSource` and writing to `connDestination` (4).
//		- Once this call returns and the function returns, each connection’s Close method runs, which causes `io.Copy` to return, terminating its goroutine (3).
//		- As a result, the data is proxied between network connections as if they had a direct connection to one another.
// - NOTE:
// 		- Since Go version 1.11, if you use `io.Copy` or `io.CopyN` when the source and destination are both `*net.TCPConn` objects,
//		- the data never enters the user space on Linux, thereby causing the data transfer to occur more efficiently.
//		- Think of it as the Linux kernel reading from one socket and writing to the other without the data needing to interact directly with your Go code.
//		- `io.CopyN` functions like `io.Copy` except it copies up to n bytes. We’ll use `io.CopyN` in the next chapter.

// What is the purpose of proxyConn?
// 	- The function takes two addresses:
// 		- source (e.g. server A)
// 		- destination (e.g. server B)
// 		- and does something like this:
// 			- Anything from A → goes to B
// 			- Anything from B → goes to A
// 		- This means that A and B are directly connected, but in fact the data is passed through this proxy.

func proxyConn(source, destination string) error {

	// 1) Connect to Source
	// 	- Here the proxy connects to the source address.
	//		- connSource is a TCP connection that can:
	// 			- Read
	// 			- Write
	// 		- defer Close means that when the function is finished, this connection is closed.

	connSource, err := net.Dial("tcp", source) // (1)
	if err != nil {
		return err
	}
	defer connSource.Close()

	// 2) Connecting to Destination (2)
	// 	- As before, it creates a second connection to the destination.
	// 	- Now the proxy has two wires:
	// 		- One wire to the source
	// 		- One wire to the destination

	connDestination, err := net.Dial("tcp", destination) // (2)
	if err != nil {
		return err
	}
	defer connDestination.Close()

	// 3) Launch the return path (Destination → Source) in a goroutine (3)
	// 	- This line is very important. The meaning of `io.Copy(dst, src)` is:
	// 		- Read from src
	// 		- Write whatever you read to dst
	// 		- Until src is over (EOF) or an error occurs
	// 		- So here:
	// 			- src = connDestination
	// 			- dst = connSource
	//		- That is:
	//			- Whatever comes from Destination, send to Source, This path is usually “responses”)
	//	- Why goroutine?
	// 		- Because the outgoing path needs to run at the same time.
	// 		- If you didn't make this a goroutine, the program would get stuck in this copy and the second path would never run.

	go func() { _, _ = io.Copy(connSource, connDestination) }() // connDestination replies to connSource (3)

	// 4) Outbound path (Source → Destination) in the main thread
	// 	- Here:
	// 		- src = connSource
	// 		- dst = connDestination
	// 		- That is:
	//			- Whatever comes from Source, send to Destination, (usually “requests”)
	// 		- This `io.Copy` continues until:
	// 			- One of the connections is closed
	// 			- or an error occurs
	// 			- When this Copy is finished, the function reaches return err.

	_, err = io.Copy(connDestination, connSource) // (4) connSource message to connDestination

	return err
}

// 5) Why doesn't a goroutine leak?
//	- Because when the function finishes:
//		- defer `connSource.Close()` is executed
// 		- defer `connDestination.Close()` is executed
// 		- When one of these connections is closed, the `io.Copy` that is running in the goroutine will also:
// 			- either get `EOF`
// 			- or get an error
// 			- and terminate.
// 			- So the goroutine dies on its own.

// 6) Why is `io.Copy` so great for proxies?
// 	- Because you don't have to:
// 		- Create your own buffer
// 		- Write a Read/Write loop
// 		- Handle errors and EOF
// 		- `io.Copy` does all of this.

// 7) Tip about Go 1.11 and Linux
// 	- If both sides are truly `*net.TCPConn`, Go (on some systems like Linux) can use kernel features to:
// 		- Less data goes into the "Go program space"
// 		- and transfers are faster and cheaper (zero-copy / splice-like)
// 		- This means better performance.
