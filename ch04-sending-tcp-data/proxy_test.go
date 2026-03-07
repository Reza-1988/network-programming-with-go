package ch04

import "io"

// Listing 4-15: Proxy data between a reader and writer

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
