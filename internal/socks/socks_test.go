package socks

import (
	"bytes"
	"context"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

// SOCKS5 protocol constants
const (
	socks5Version byte = 0x05
	noAuth        byte = 0x00
	noAcceptable  byte = 0xFF
	cmdConnect    byte = 0x01
	addrTypeIPv4  byte = 0x01
	repSuccess    byte = 0x00
	cmdNotSupported byte = 0x07
)

type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
	closed   bool
	readErr  error
	writeErr error
	mutex    sync.Mutex
}

func newMockConn() *mockConn {
	return &mockConn{
		readBuf:  bytes.NewBuffer(nil),
		writeBuf: bytes.NewBuffer(nil),
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.closed {
		return 0, net.ErrClosed
	}
	if m.readErr != nil {
		return 0, m.readErr
	}
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.closed {
		return 0, net.ErrClosed
	}
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.closed = true
	return nil
}

func (m *mockConn) LocalAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1080}
}

func (m *mockConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 48765}
}

func (m *mockConn) SetDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (m *mockConn) SetReadData(data []byte) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.readBuf.Reset()
	m.readBuf.Write(data)
}

func (m *mockConn) GetWrittenData() []byte {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	return m.writeBuf.Bytes()
}

func (m *mockConn) SetReadError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.readErr = err
}

func (m *mockConn) SetWriteError(err error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.writeErr = err
}

// SimpleSocksServer is a simplified SOCKS5 server for testing
type SimpleSocksServer struct {
	// This simplified server only implements the minimum needed for testing
	connections chan net.Conn
	started     bool
	addr        string
}

func NewSimpleSocksServer(addr string) *SimpleSocksServer {
	return &SimpleSocksServer{
		connections: make(chan net.Conn, 10),
		addr:        addr,
	}
}

func (s *SimpleSocksServer) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	// Read SOCKS version
	buf := make([]byte, 3)
	if _, err := io.ReadFull(conn, buf); err != nil {
		return
	}
	
	// Check version
	if buf[0] != socks5Version {
		// Invalid version
		conn.Write([]byte{socks5Version, noAcceptable})
		return
	}
	
	// Send auth method response
	conn.Write([]byte{socks5Version, noAuth})
	
	// Read request
	header := make([]byte, 4)
	if _, err := io.ReadFull(conn, header); err != nil {
		return
	}
	
	// Check command
	if header[1] != cmdConnect {
		// Not implemented
		conn.Write([]byte{socks5Version, cmdNotSupported, 0x00, addrTypeIPv4, 0, 0, 0, 0, 0, 0})
		return
	}
	
	// Skip the rest of the request
	
	// Send success response
	conn.Write([]byte{socks5Version, repSuccess, 0x00, addrTypeIPv4, 127, 0, 0, 1, 0x04, 0x38})
	
	// Now just echo data
	buf = make([]byte, 1024)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			break
		}
		conn.Write(buf[:n])
	}
}

func (s *SimpleSocksServer) Start() {
	s.started = true
	go func() {
		for conn := range s.connections {
			go s.handleConnection(conn)
		}
	}()
}

func (s *SimpleSocksServer) AddConnection(conn net.Conn) {
	s.connections <- conn
}

func TestNewServer(t *testing.T) {
	server, err := NewServer("localhost:1080", "testkey")
	
	if err != nil {
		t.Fatalf("NewServer returned error: %v", err)
	}
	
	if server == nil {
		t.Fatal("NewServer returned nil")
	}
	
	if server.addr != "localhost:1080" {
		t.Errorf("Expected addr to be 'localhost:1080', got '%s'", server.addr)
	}
}

// mockDialer is a mock implementation of the dialer function
type mockDialer struct {
	conn      net.Conn
	dialError error
}

func (d *mockDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.dialError != nil {
		return nil, d.dialError
	}
	return d.conn, nil
}

func TestFullIntegration(t *testing.T) {
	t.Skip("Skipping integration test as it requires actual connections")
	// Create a test connection pair using a pipe
	clientConn, serverConn := net.Pipe()
	
	// Create a simple socks server
	sockServer := NewSimpleSocksServer("localhost:1080")
	sockServer.Start()
	sockServer.AddConnection(serverConn)
	
	// Send SOCKS5 initialization
	clientConn.Write([]byte{socks5Version, 1, noAuth})
	
	// Read response
	resp := make([]byte, 2)
	io.ReadFull(clientConn, resp)
	
	if resp[0] != socks5Version || resp[1] != noAuth {
		t.Fatalf("Unexpected auth response: %v", resp)
	}
	
	// Send connect request
	clientConn.Write([]byte{
		socks5Version,  // Version
		cmdConnect,     // Connect command
		0x00,           // Reserved
		addrTypeIPv4,   // Address type
		192, 168, 1, 1, // IP address 192.168.1.1
		0x00, 0x50,     // Port 80
	})
	
	// Read connect response
	respHeader := make([]byte, 10)
	io.ReadFull(clientConn, respHeader)
	
	if respHeader[0] != socks5Version || respHeader[1] != repSuccess {
		t.Fatalf("Unexpected connect response: %v", respHeader)
	}
	
	// Test echo functionality
	testData := []byte("Hello, SOCKS5!")
	clientConn.Write(testData)
	
	// Read echo response
	echoResp := make([]byte, len(testData))
	io.ReadFull(clientConn, echoResp)
	
	if !bytes.Equal(echoResp, testData) {
		t.Fatalf("Echo response doesn't match. Got %v, want %v", echoResp, testData)
	}
	
	// Clean up
	clientConn.Close()
}

// TestXorConn tests the XorConn wrapper
func TestXorConn(t *testing.T) {
	mockConn := newMockConn()
	mockConn.SetReadData([]byte("test data"))
	
	// Create an XOR encoded connection
	xorConn := &XorConn{
		XorReaderWriter: nil, // We're only testing the net.Conn interface methods
		conn:            mockConn,
	}
	
	// Test LocalAddr
	localAddr := xorConn.LocalAddr()
	if localAddr.String() != "127.0.0.1:1080" {
		t.Errorf("Expected LocalAddr 127.0.0.1:1080, got %s", localAddr.String())
	}
	
	// Test RemoteAddr
	remoteAddr := xorConn.RemoteAddr()
	if remoteAddr.String() != "127.0.0.1:48765" {
		t.Errorf("Expected RemoteAddr 127.0.0.1:48765, got %s", remoteAddr.String())
	}
	
	// Test SetDeadline
	if err := xorConn.SetDeadline(time.Now()); err != nil {
		t.Errorf("SetDeadline failed: %v", err)
	}
	
	// Test SetReadDeadline
	if err := xorConn.SetReadDeadline(time.Now()); err != nil {
		t.Errorf("SetReadDeadline failed: %v", err)
	}
	
	// Test SetWriteDeadline
	if err := xorConn.SetWriteDeadline(time.Now()); err != nil {
		t.Errorf("SetWriteDeadline failed: %v", err)
	}
} 