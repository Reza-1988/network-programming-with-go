package ch04

import (
	"io"
	"net"
	"sync"
	"testing"
)

// Listing 4-15: Proxy data between a reader and writer
// 	- This proxy function (1) is a bit more useful in that it accepts the ubiquitous `io.Reader` and `io.Writer` interfaces instead of `net.Conn`.
//	- Because of this change, you could proxy data from a network connection to `os.Stdout`, `*bytes.Buffer`, `*os.File`, or any number of objects that implement the `io.Writer `interface.
// 	- Likewise, you could read bytes from any object that implements the `io.Reader` interface and send them to the writer.
//	- This implementation of proxy supports replies if the from reader implements the `io.Writer` interface and the to writer implements the `io.Reader `interface.

func proxy(from io.Reader, to io.Writer) error {
	fromWriter, fromIsWriter := from.(io.Writer)
	toReader, toIsReader := to.(io.Reader)

	if toIsReader && fromIsWriter {
		// Send replies since "from" and "to" implement the
		// necessary interfaces.
		go func() { _, _ = io.Copy(fromWriter, toReader) }()
	}

	_, err := io.Copy(to, from)
	return err
}

// Listing 4-16: Creating the listener
// 	- You start by initializing a server (1) that listens for incoming connections.
//	- It reads bytes from each connection, replies with the string "pong" when it receives the string "ping," and echoes any other message it receives.

func TestProxy(t *testing.T) {
	var wg sync.WaitGroup

	// server listens for a "ping" message and responds with a
	// "pong" message. All other messages are echoed back to the client.
	server, err := net.Listen("tcp", "127.0.0.1:") // (1)
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			conn, err := server.Accept()
			if err != nil {
				return
			}

			go func(c net.Conn) {
				defer c.Close()

				for {
					buf := make([]byte, 1024)
					n, err := c.Read(buf)
					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}

						return
					}

					switch msg := string(buf[:n]); msg {
					case "ping":
						_, err = c.Write([]byte("pong"))
					default:
						_, err = c.Write(buf[:n])
					}

					if err != nil {
						if err != io.EOF {
							t.Error(err)
						}

						return
					}
				}
			}(conn)
		}
	}()

	// Listing 4-17: Set up the proxy between the client and server
	// 	- You then set up a proxy server (1) that handles the message passing between the client and the destination server.
	//	- The proxy server listens for incoming client connections. Once a client connection accepts (2),
	//    the proxy establishes a connection to the destination server (3) and starts proxying messages (4).
	//  - Since the proxy server passes two `net.Conn` objects to proxy, and `net.Conn` implements the `io.ReadWriter` interface, the server proxies replies automatically.
	// 	- Then `io.Copy` writes to the Write method of the destination `net.Conn` everything it reads from the Read method of the origin `net.Conn`,
	//	  and vice versa for replies from the destination to the origin.

	// proxyServer proxies messages from client connections to the
	// destinationServer. Replies from the destinationServer are proxied
	// back to the clients.
	proxyServer, err := net.Listen("tcp", "127.0.0.1:") // (1)
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			conn, err := proxyServer.Accept() // (2)
			if err != nil {
				return
			}

			go func(from net.Conn) {
				defer from.Close()
				to, err := net.Dial("tcp", // (3)
					server.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}
				defer to.Close()

				err = proxy(from, to) // (4)
				if err != nil && err != io.EOF {
					t.Error(err)
				}
			}(conn)
		}
	}()

	// Listing 4-18: Proxying data from an upstream server to a downstream server
	// 	- You run the proxy through a series of tests (1) to verify that your ping messages result in pong replies and that the destination echoes anything else you send.
	// 	  The output should look like the following:
	//	$ go test -race -v proxy_test.go // (1)
	//  === RUN TestProxy
	//	--- PASS: TestProxy (0.00s)
	//		proxy_test.go:138: "ping" -> proxy -> "pong"
	//		proxy_test.go:138: "pong" -> proxy -> "pong"
	//		proxy_test.go:138: "echo" -> proxy -> "echo"
	//		proxy_test.go:138: "ping" -> proxy -> "pong"
	//	PASS
	//	ok command-line-arguments 1.018s
	// 	- I’m in the habit of running my tests with the `-race` flag (1) to enable the race detector.
	//	  The race detector can help alert you to data races that need your attention.
	//	  Although not necessary for this test, enabling it is a good habit.

	conn, err := net.Dial("tcp", proxyServer.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	msgs := []struct{ Message, Reply string }{
		{"ping", "pong"},
		{"pong", "pong"},
		{"echo", "echo"},
		{"ping", "pong"},
	}

	for i, m := range msgs {
		_, err = conn.Write([]byte(m.Message))
		if err != nil {
			t.Fatal(err)
		}

		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			t.Fatal(err)
		}

		if actual := string(buf[:n]); actual != m.Reply {
			t.Errorf("%d: expected reply: %q; actual: %q",
				i, m.Reply, actual)
		}
	}

	_ = conn.Close()
	_ = proxyServer.Close()
	_ = server.Close()
	wg.Wait()
}
