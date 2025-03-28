// Package socks provides SOCKS5 proxy functionality
package socks

import (
	"context"
	"log"
	"net"
	"time"

	"github.com/armon/go-socks5"
	
	"file-sharing-utility/internal/xorrw"
)

// XorConn wraps an XorReaderWriter to implement the full net.Conn interface
type XorConn struct {
	*xorrw.XorReaderWriter
	conn net.Conn
}

// LocalAddr returns the local network address.
func (x *XorConn) LocalAddr() net.Addr {
	return x.conn.LocalAddr()
}

// RemoteAddr returns the remote network address.
func (x *XorConn) RemoteAddr() net.Addr {
	return x.conn.RemoteAddr()
}

// SetDeadline sets the read and write deadlines.
func (x *XorConn) SetDeadline(t time.Time) error {
	return x.conn.SetDeadline(t)
}

// SetReadDeadline sets the read deadline.
func (x *XorConn) SetReadDeadline(t time.Time) error {
	return x.conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the write deadline.
func (x *XorConn) SetWriteDeadline(t time.Time) error {
	return x.conn.SetWriteDeadline(t)
}

// Server represents a SOCKS5 proxy server
type Server struct {
	server *socks5.Server
	addr   string
}

// NewServer creates a new SOCKS5 server with the given address and XOR key
func NewServer(addr, xorKey string) (*Server, error) {
	// Create a new SOCKS5 configuration
	conf := &socks5.Config{}
	
	// Apply XOR encoding/decoding if a key is provided
	if xorKey != "" {
		// Custom dial function to apply XOR encoding
		conf.Dial = func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Connect to the target server
			conn, err := net.Dial(network, addr)
			if err != nil {
				return nil, err
			}
			
			// Wrap the connection with XOR encoding
			xorRW := xorrw.NewXorReaderWriter(conn, []byte(xorKey))
			// Wrap with full net.Conn implementation
			xorConn := &XorConn{
				XorReaderWriter: xorRW,
				conn:            conn,
			}
			return xorConn, nil
		}
	}
	
	// Create SOCKS5 server
	server, err := socks5.New(conf)
	if err != nil {
		return nil, err
	}
	
	return &Server{
		server: server,
		addr:   addr,
	}, nil
}

// Start starts the SOCKS5 server
func (s *Server) Start() error {
	log.Printf("Starting SOCKS5 server on %s", s.addr)
	return s.server.ListenAndServe("tcp", s.addr)
}

// StartAsync starts the SOCKS5 server in a goroutine
func (s *Server) StartAsync() {
	go func() {
		if err := s.Start(); err != nil {
			log.Fatalf("SOCKS5 server error: %v", err)
		}
	}()
} 