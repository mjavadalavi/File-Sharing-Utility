package xorrw

import (
	"bytes"
	"io"
	"testing"
)

func TestXorReaderWriter(t *testing.T) {
	// Test data
	originalData := []byte("Hello, this is a test message for XOR encoding.")
	key := []byte("secretkey")
	
	// Create buffer for testing
	buffer := &bytes.Buffer{}
	
	// Create XorReaderWriter
	xorRW := NewXorReaderWriter(buffer, key)
	
	// Test writing (encoding)
	n, err := xorRW.Write(originalData)
	if err != nil {
		t.Fatalf("Error writing to XorReaderWriter: %v", err)
	}
	if n != len(originalData) {
		t.Fatalf("Wrong number of bytes written: got %d, want %d", n, len(originalData))
	}
	
	// Encoded data should be different from original
	encodedData := buffer.Bytes()
	if bytes.Equal(encodedData, originalData) {
		t.Fatalf("Encoded data is the same as original, XOR encoding failed")
	}
	
	// Create a new buffer with the encoded data
	encodedBuffer := bytes.NewBuffer(encodedData)
	
	// Create a new XorReaderWriter for decoding
	decodeXorRW := NewXorReaderWriter(encodedBuffer, key)
	
	// Read (decode) the data
	decodedData := make([]byte, len(originalData))
	n, err = decodeXorRW.Read(decodedData)
	if err != nil && err != io.EOF {
		t.Fatalf("Error reading from XorReaderWriter: %v", err)
	}
	if n != len(originalData) {
		t.Fatalf("Wrong number of bytes read: got %d, want %d", n, len(originalData))
	}
	
	// Decoded data should match original
	if !bytes.Equal(decodedData, originalData) {
		t.Fatalf("Decoded data doesn't match original.\nOriginal: %v\nDecoded: %v", originalData, decodedData)
	}
}

func TestXorReaderWriterWithEmptyKey(t *testing.T) {
	// Test data
	originalData := []byte("Hello, this is a test message.")
	emptyKey := []byte{}
	
	// Create buffer for testing
	buffer := &bytes.Buffer{}
	
	// This should panic because of division by zero when empty key is used
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("Expected panic with empty key, but no panic occurred")
		}
	}()
	
	// Create XorReaderWriter with empty key - should cause panic
	xorRW := NewXorReaderWriter(buffer, emptyKey)
	xorRW.Write(originalData)
}

// MockReadWriter implements io.ReadWriter for testing
type MockReadWriter struct {
	ReadData  []byte
	WriteData []byte
	ReadErr   error
	WriteErr  error
}

func (m *MockReadWriter) Read(p []byte) (int, error) {
	if m.ReadErr != nil {
		return 0, m.ReadErr
	}
	return copy(p, m.ReadData), nil
}

func (m *MockReadWriter) Write(p []byte) (int, error) {
	if m.WriteErr != nil {
		return 0, m.WriteErr
	}
	m.WriteData = append(m.WriteData, p...)
	return len(p), nil
}

func TestXorReaderWriterWithErrors(t *testing.T) {
	// Test data
	originalData := []byte("Hello, this is a test message.")
	key := []byte("secretkey")
	
	// Test read error
	mockRW := &MockReadWriter{
		ReadErr: io.ErrUnexpectedEOF,
	}
	
	xorRW := NewXorReaderWriter(mockRW, key)
	buf := make([]byte, 10)
	_, err := xorRW.Read(buf)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Expected ErrUnexpectedEOF, got %v", err)
	}
	
	// Test write error
	mockRW = &MockReadWriter{
		WriteErr: io.ErrShortWrite,
	}
	
	xorRW = NewXorReaderWriter(mockRW, key)
	_, err = xorRW.Write(originalData)
	if err != io.ErrShortWrite {
		t.Errorf("Expected ErrShortWrite, got %v", err)
	}
}

// MockCloser is a mock ReadWriteCloser for testing Close method
type MockCloser struct {
	MockReadWriter
	CloseCalled bool
	CloseErr    error
}

func (m *MockCloser) Close() error {
	m.CloseCalled = true
	return m.CloseErr
}

func TestXorReaderWriterClose(t *testing.T) {
	// Test closing with an underlying closer
	mockCloser := &MockCloser{}
	xorRW := NewXorReaderWriter(mockCloser, []byte("key"))
	
	err := xorRW.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !mockCloser.CloseCalled {
		t.Errorf("Close was not called on the underlying closer")
	}
	
	// Test closing with error
	mockCloser = &MockCloser{
		CloseErr: io.ErrClosedPipe,
	}
	xorRW = NewXorReaderWriter(mockCloser, []byte("key"))
	
	err = xorRW.Close()
	if err != io.ErrClosedPipe {
		t.Errorf("Expected ErrClosedPipe, got %v", err)
	}
	
	// Test closing without an underlying closer
	buffer := &bytes.Buffer{} // doesn't implement Closer
	xorRW = NewXorReaderWriter(buffer, []byte("key"))
	
	err = xorRW.Close()
	if err != nil {
		t.Errorf("Expected nil error for non-closer, got %v", err)
	}
} 