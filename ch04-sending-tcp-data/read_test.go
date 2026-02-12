package ch04

import (
	"crypto/rand"
	"io"
	"net"
	"testing"
)

// --- STEP1 ---
// ## Reading Data into a Fixed Buffer
// TCP connections in Go implement the `io.Reader` interface, which allows you to read data from the network connection.
//   - To read data from a network connection, you need to provide a buffer for the network connection’s Read method to fill.
//   - The Read method will populate the buffer to its capacity if there is enough data in the connection’s receive buffer.
//   - If there are fewer bytes in the receive buffer than the capacity of the buffer you provide,
//   - Read will populate the given buffer with the data and return instead of waiting for more data to arrive.
//   - In other words, Read is not guaranteed to fill your buffer to capacity before it returns.
//   - Listing 4-1 demonstrates the process of reading data from a network connection into a byte slice.
//
// Listing 4-1: Receiving data over a network connection
func TestReadIntoBuffer(t *testing.T) {
	payload := make([]byte, 1<<24) // 16 MB
	_, err := rand.Read(payload)   // generate a random payload
	if err != nil {
		t.Fatal(err)
	}

	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Log(err)
			return
		}
		defer conn.Close()

		_, err = conn.Write(payload)
		if err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1<<19) // 512 KB

	for {
		n, err := conn.Read(buf)
		if err != nil {
			if err != io.EOF {
				t.Error(err)
			}
			break
		}

		t.Logf("read %d bytes", n) // buf[:n] is the data read from conn
	}

	conn.Close()
}
