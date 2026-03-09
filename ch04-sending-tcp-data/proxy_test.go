package ch04

import "io"

// Listing 4-15: Proxy data between a reader and writer
// 	- This proxy function (1) is a bit more useful in that it accepts the ubiquitous `io.Reader` and `io.Writer` interfaces instead of `net.Conn`.
//	- Because of this change, you could proxy data from a network connection to `os.Stdout`, `*bytes.Buffer`, `*os.File`, or any number of objects that implement the `io.Writer `interface.
// 	- Likewise, you could read bytes from any object that implements the `io.Reader` interface and send them to the writer.
//	- This implementation of proxy supports replies if the from reader implements the `io.Writer` interface and the to writer implements the `io.Reader `interface.

func proxy(from io.Reader, to io.Writer) error { // (1)
	fromWriter, fromIsWriter := from.(io.Writer)
	toReader, toIsReader := to.(io.Reader)

	if toIsReader && fromIsWriter {
		//Send replies since "form" and to Implement the necessary interface.
		go func() { _, _ = io.Copy(fromWriter, toReader) }()
	}
	_, err := io.Copy(to, from)
	return err
}
