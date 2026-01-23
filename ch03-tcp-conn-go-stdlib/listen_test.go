package ch03

import (
	"net"
	"testing"
)

// ## Binding, Listening for, and Accepting Connections
// To create a TCP server capable of listening for incoming connections (called a listener), use the `net.Listen` function.
// This function will return an object that implements the `net.Listener` interface.

// ### Listing 3-1: Creating a listener on 127.0.0.1 using a random port.
func TestListener(t *testing.T) {
	// `net.Listen`
	// 		- The `net.Listen` function accepts a network type ("tcp") and
	//        an IP address and port separated by a colon "127.0.0.1:0"
	// 		- The `net.Listen` function returns a `net.Listener` interface  and an `error` interface.
	// 			- If the function returns successfully, the listener is bound to the specified IP address and port.
	//			- Binding means that the operating system has exclusively assigned the port on the given IP address to the listener.
	//			- The operating system allows no other processes to listen for incoming traffic on bound ports.
	//			- If you attempt to bind a listener to a currently bound port, `net.Listen` will return an error.
	//		- You can choose to leave the IP address and port parameters empty.
	//			- If the port is zero or empty, Go will randomly assign a port number to your listener.
	//			- You can retrieve the listener’s address by calling its `Addr()` method.
	//			- Likewise, if you omit the IP address, your listener will be bound to all unicast and anycast IP addresses on the system.
	//			- Omitting both the IP address and port, or passing in a colon(":") for the second argument to `net.Listen`,
	//			  will cause your listener to bind to all unicast and anycast IP addresses using a random port.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	// You should always be diligent about closing your listener gracefully by calling its `Close()` method,
	// often in a `defer` if it makes sense for your code.
	// Granted, this is a test case, and Go will tear down the listener when the test completes, but it’s good practice nonetheless.
	// Failure to close the listener may lead to memory leaks or deadlocks in your code, because calls to the listener’s Accept method may block indefinitely.
	// Closing the listener immediately unblocks calls to the Accept method.
	defer func() { _ = listener.Close() }()
	t.Logf("bond to %q", listener.Addr())

	// ------

	// ### Listing 3-2: Accepting and handling incoming TCP connection requests
	// 	- Unless you want to accept only a single incoming connection,
	//		- you need to use a for loop so your server will accept each incoming connection,
	// 	      handle it in a goroutine, and loop back around, ready to accept the next connection.
	// 		- Serially accepting connections is perfectly acceptable and efficient,
	//		  but beyond that point, you should use a goroutine to handle each connection.
	// 		- You could certainly write serialized code after accepting a connection if your use case demands it,
	//   	  but it would be woefully inefficient and fail to take advantage of Go’s strengths.
	for {
		// We start the for loop by calling the listener’s `Accept()` method.
		// 	- The listener `Accept()` method will block until the listener detects an incoming connection and
		//	  completes the TCP handshake process between the client and the server.
		// 	- The call returns a `net.Conn` interface and an `error`.
		// 		- If the handshake failed or the listener closed, for example, the error interface would be non-nil.
		//	- The connection interface’s underlying type is a pointer to a `net.TCPConn` object because you’re accepting TCP connections.
		// 		- The connection interface represents the server’s side of the TCP connection.
		//		- In most cases, `net.Conn` provides all methods you’ll need for general interactions with the client.
		//		- However, the `net.TCPConn` object provides additional functionality we’ll cover in Chapter 4 should you require more control.
		conn, err := listener.Accept()
		if err != nil {
			t.Fatal(err)
		}
		// To concurrently handle client connections,
		// you spin off a goroutine to asynchronously handle each connection so your listener can ready itself for the next incoming connection.
		// Then you call the connection’s Close method before the goroutine exits to gracefully terminate the connections by sending a FIN packet to the server.
		go func(c net.Conn) {
			defer c.Close()

			// Your code would handle the connection here.
		}(conn)
	}
}
