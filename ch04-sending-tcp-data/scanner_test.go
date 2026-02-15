package ch04

import (
	"bufio"
	"net"
	"reflect"
	"testing"
)

// --- STEP2 ---
// ## Delimited Reading by Using a Scanner
// - Reading data from a network connection by using the method I just showed means your code needs to make sense of the data it receives.
//   - Since TCP is a stream-oriented protocol, a client can receive a stream of bytes across many packets.
//   - Unlike sentences, binary data doesn’t include inherent punctuation that tells you where one message starts and stops.
// - If, for example, your code is reading a series of email messages from a server,
//   - your code will have to inspect each byte for delimiters indicating the boundaries of each message in the stream of bytes.
//   - Alternatively, your client may have an established protocol with the server whereby the server sends a fixed number of bytes to indicate the payload size the server will send next.
//   - Your code can then use this size to create an appropriate buffer for the payload.
// - However, if you choose to use a delimiter to indicate the end of one message and the beginning of another,
//   - writing code to handle edge cases isn’t so simple.
//   - For example, you may read 1KB of data from a single Read on the network connection and find that it contains two delimiters.
//   - This indicates that you have two complete messages, but you don’t have enough information about the chunk of data following the second delimiter to know whether it is also a complete message.
//   - If you read another 1KB of data and find no delimiters, you can conclude that this entire block of data is a continuation of the last message in the previous 1KB you read.
//   - But what if you read 1KB of nothing but delimiters?
// - If this is starting to sound a bit complex, it’s because you must account for data across multiple Read calls and handle any errors along the way.
//   - Anytime you’re tempted to roll your own solution to such a problem, check the standard library to see if a tried-and-true implementation already exists.
//   - In this case, `bufio.Scanner` does what you need.
// - The `bufio.Scanner` is a convenient bit of code in Go’s standard library that allows you to read delimited data.
//	- The Scanner accepts an `io.Reader` as its input.
//	- Since `net.Conn` has a Read method that implements the `io.Reader` interface, you can use the Scanner to easily read delimited data from a network connection.
//	- Listing 4-2 sets up a listener to serve up delimited data for later parsing by `bufio.Scanner`.

// Listing 4-2: Creating a test to serve up a constant payload
//   - This listener should look familiar by now.
//   - All it’s meant to do is serve up the payload 1.
//   - Listing 4-3 uses `bufio.Scanner` to read a string from the network, splitting each chunk by whitespace.
const payload = "The bigger the interface, the weaker the abstraction."

func TestScanner(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:") // (1)
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

	// Listing 4-3: Using bufio.Scanner to read whitespace-delimited text from the network

	// 1) The client connects to the server
	// 	- This line is like saying: "Connect to the same server we listened to above."
	// 	- The output is conn (client-side TCP connection)
	conn, err := net.Dial("tcp", listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// 2) Creating a Scanner on a Network Connection
	// 	- This means: "Create a scanner that reads data from conn."
	//	- The scanner itself reads from the network behind the scenes with Read,
	//	- but you don't have to worry about `Read(buf)` and delimiters.
	scanner := bufio.NewScanner(conn) // (1)

	// 3) Determine the data separation method: word by word
	// 	- The Scanner needs to know what “each piece” means.
	// 	- ScanWords says: “Give me one word at a time.”
	// 	- A word means: anything between spaces (space, \n, \t, etc.).
	// 	- So this payload: "The bigger the interface, the weaker the abstraction."
	// 	- It is broken down into words like: "The" "bigger" "the" "interface," ...
	// 	- Note: interface retains the comma because ScanWords only recognizes whitespace as a separator, not punctuation.
	scanner.Split(bufio.ScanWords)

	// 4) A slice to collect words
	// 	- We create an empty slice of string to store the words that Scanner returns.
	var words []string

	// 5) Word Reading Circle
	// 	- This is the most important part:
	//		- `scanner.Scan()` tries to prepare the next word each time.
	// 		- If it finds a word → it returns true and the loop continues
	// 		- If the data runs out or an error occurs → it returns false and the loop ends
	// 		- `scanner.Text()` returns the word that was found this time as a string.
	// 		- append also adds that word to the words list.
	// 		- So the output of words is a list of all the words in the text.
	for scanner.Scan() { // (2)
		words = append(words, scanner.Text()) // (3)
	}

	// 6) Checking Scanner Errors
	// 	- When the loop ends, we don't know why:
	// 	- Did the normal data run out?
	// 	- Or did an error occur?
	// 	- `scanner.Err()` returns if there was an error, otherwise nil.
	err = scanner.Err()
	if err != nil {
		t.Error(err)
	}

	// 7) Comparison with what we expect
	// 	- This is the list of words we expect Scanner to produce.
	expected := []string{"The", "bigger", "the", "interface,", "the",
		"weaker", "the", "abstraction."}

	// 	- `DeepEqual` checks whether two slices are exactly the same in terms of content.
	// 	- If they are not the same, it means the Scanner did not read them correctly or the segmentation was different → the test fails.
	if !reflect.DeepEqual(words, expected) {
		t.Fatal("inaccurate scanned word list")
	}

	// 8) Print the result to see
	// 	- It's just for logging into the test to see what happened.
	t.Logf("Scanned words: %#v", words) // (4)
}

// 	- Since you know you’re reading a string from the server, you start by creating a `bufio.Scanner` that reads from the network connection (1).
// 		- By default, the scanner will split data read from the network connection when it encounters a newline character (\n) in the stream of data.
//		- Instead, you elect to have the scanner delimit the input at the end of each word by using `bufio.ScanWords`,
//		- which will split the data when it encounters a word border, such as whitespace or sentence-terminating punctuation.
// 	- You keep reading data from the scanner as long as it tells you it’s read data from the connection (2).
//		- Every call to Scan can result in multiple calls to the network connection’s Read method until the scanner finds its delimiter or reads an error from the connection.
// 		- It hides the complexity of searching for a delimiter across one or more reads from the network connection and returning the resulting messages.
// 	- The call to the scanner’s Text method returns the chunk of data as a string—a single word and adjacent punctuation, in this case—that it just read from the network connection (3).
//		- The code continues to iterate around the for loop until the scanner receives an `io.EOF` or other error from the network connection.
//		- If it’s the latter, the scanner’s `Err` method will return a non-nil error. You can view the scanned words (4) by adding the `-v` flag to the go test command.

// Big different from Step 1
// In the previous example you used `conn.Read(buf)` directly, so you created a buffer (container) to read from.
// Here you don't read directly; you use `bufio.Scanner`. The scanner itself does the following behind the scenes:
// 	- It has an internal buffer
// 	- It performs several reads
// 	- It collects the data
// 	- Until a separator (e.g. a word boundary) is found
// 	- Then it gives you the result with `scanner.Text()`
// 	- So you didn't "create" the buffer because the Scanner manages the buffer itself.
