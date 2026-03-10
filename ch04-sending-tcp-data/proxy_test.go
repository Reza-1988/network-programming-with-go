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

	// proxyServer proxies messages from client connections to the
	// destinationServer. Replies from the destinationServer are proxied
	// back to the clients.
	proxyServer, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		for {
			conn, err := proxyServer.Accept()
			if err != nil {
				return
			}

			go func(from net.Conn) {
				defer from.Close()
				to, err := net.Dial("tcp",
					server.Addr().String())
				if err != nil {
					t.Error(err)
					return
				}
				defer to.Close()

				err = proxy(from, to)
				if err != nil && err != io.EOF {
					t.Error(err)
				}
			}(conn)
		}
	}()