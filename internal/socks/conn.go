package socks

import (
	"net"
	
	"file-sharing-utility/internal/xorrw"
)

// Connection represents a SOCKS5 connection
type Connection struct {
	net.Conn
	xorConn *xorrw.XorReaderWriter
}

// NewConnection creates a new connection with optional XOR encoding
func NewConnection(conn net.Conn, xorKey string) net.Conn {
	if xorKey == "" {
		return conn
	}
	
	xorConn := xorrw.NewXorReaderWriter(conn, []byte(xorKey))
	return &Connection{
		Conn:    conn,
		xorConn: xorConn,
	}
}

// Read reads data from the connection with XOR decoding
func (c *Connection) Read(b []byte) (int, error) {
	if c.xorConn != nil {
		return c.xorConn.Read(b)
	}
	return c.Conn.Read(b)
}

// Write writes data to the connection with XOR encoding
func (c *Connection) Write(b []byte) (int, error) {
	if c.xorConn != nil {
		return c.xorConn.Write(b)
	}
	return c.Conn.Write(b)
}

// Close closes the connection
func (c *Connection) Close() error {
	if c.xorConn != nil {
		return c.xorConn.Close()
	}
	return c.Conn.Close()
} 