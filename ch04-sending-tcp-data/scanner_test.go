package ch04

import (
	"bufio"
	"net"
	"reflect"
	"testing"
)

// --- STEP2---
// ## Delimited Reading by Using a Scanner
// - Reading data from a network connection by using the method I just showed means your code needs to make sense of the data it receives.
//   - Since TCP is a stream-oriented protocol, a client can receive a stream of bytes across many packets.
//   - Unlike sentences, binary data doesn’t include inherent punctuation that tells you where one message starts and stops.
//
// - If, for example, your code is reading a series of email messages from a server,
//   - your code will have to inspect each byte for delimiters indicating the boundaries of each message in the stream of bytes.
//   - Alternatively, your client may have an established protocol with the server whereby the server sends a fixed number of bytes to indicate the payload size the server will send next.
//   - Your code can then use this size to create an appropriate buffer for the payload.
//
// - However, if you choose to use a delimiter to indicate the end of one message and the beginning of another,
//   - writing code to handle edge cases isn’t so simple.
//   - For example, you may read 1KB of data from a single Read on the network connection and find that it contains two delimiters.
//   - This indicates that you have two complete messages, but you don’t have enough information about the chunk of data following the second delimiter to know whether it is also a complete message.
//   - If you read another 1KB of data and find no delimiters, you can conclude that this entire block of data is a continuation of the last message in the previous 1KB you read.
//   - But what if you read 1KB of nothing but delimiters?
const payload = "The bigger the interface, the weaker the abstraction."

func TestScanner(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:")
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			t.Error(err)
			return
		}
		defer conn.Close()

		_, err = conn.Write([]byte(payload))
		if err != nil {
			t.Error(err)
		}
	}()

	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	scanner.Split(bufio.ScanWords)

	var words []string

	for scanner.Scan() {
		words = append(words, scanner.Text())
	}

	err = scanner.Err()
	if err != nil {
		t.Error(err)
	}

	expected := []string{"The", "bigger", "the", "interface,", "the",
		"weaker", "the", "abstraction."}

	if !reflect.DeepEqual(words, expected) {
		t.Fatal("inaccurate scanned word list")
	}
	t.Logf("Scanned words: %#v", words)
}
