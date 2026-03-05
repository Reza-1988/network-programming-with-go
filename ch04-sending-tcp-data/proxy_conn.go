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

func proxyConn(source, destination string) error {
	connSource, err := net.Dial("tcp", source) // (1)
	if err != nil {
		return err
	}
	defer connSource.Close()

	connDestination, err := net.Dial("tcp", destination) // (2)
	if err != nil {
		return err
	}
	defer connDestination.Close()

	// connDestination replies to connSource
	go func() { _, _ = io.Copy(connSource, connDestination) }() // (3)

	// connSource message to connDestination
	_, err = io.Copy(connDestination, connSource) // (4)

	return err
}
