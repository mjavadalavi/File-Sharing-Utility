// Package xorrw provides XOR-based encoding and decoding for data streams
package xorrw

import (
	"io"
)

// XorReaderWriter is a wrapper around a Reader/Writer that applies XOR encoding/decoding
type XorReaderWriter struct {
	rw     io.ReadWriter // The underlying ReadWriter
	key    []byte        // XOR key
	keyPos int           // Current position in the key
}

// NewXorReaderWriter creates a new XorReaderWriter with the given ReadWriter and key
func NewXorReaderWriter(rw io.ReadWriter, key []byte) *XorReaderWriter {
	return &XorReaderWriter{
		rw:     rw,
		key:    key,
		keyPos: 0,
	}
}

// Read reads data from the underlying reader and applies XOR decoding
func (x *XorReaderWriter) Read(p []byte) (n int, err error) {
	// Read from the underlying reader
	n, err = x.rw.Read(p)
	if err != nil {
		return n, err
	}

	// Apply XOR decoding
	for i := 0; i < n; i++ {
		// XOR the current byte with the current key byte
		p[i] = p[i] ^ x.key[x.keyPos]
		
		// Move to the next key byte, wrapping around if necessary
		x.keyPos = (x.keyPos + 1) % len(x.key)
	}

	return n, nil
}

// Write writes XOR encoded data to the underlying writer
func (x *XorReaderWriter) Write(p []byte) (n int, err error) {
	// Create a copy of the data to encode
	encoded := make([]byte, len(p))
	copy(encoded, p)

	// Apply XOR encoding
	for i := 0; i < len(encoded); i++ {
		// XOR the current byte with the current key byte
		encoded[i] = encoded[i] ^ x.key[x.keyPos]
		
		// Move to the next key byte, wrapping around if necessary
		x.keyPos = (x.keyPos + 1) % len(x.key)
	}

	// Write the encoded data to the underlying writer
	return x.rw.Write(encoded)
}

// Close implements the Closer interface for cleanup
func (x *XorReaderWriter) Close() error {
	// If the underlying ReadWriter implements Closer, close it
	if closer, ok := x.rw.(io.Closer); ok {
		return closer.Close()
	}
	return nil
} 