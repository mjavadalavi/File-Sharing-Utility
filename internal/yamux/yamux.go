package yamux

import (
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"
)

// Protocol constants
const (
	// Header byte size
	headerSize = 10

	// Message types
	typeData byte = iota
	typeSYN
	typeFIN
	typeACK
	typePING
	typePONG

	// Default settings
	defaultAcceptBacklog         = 256
	defaultEnableKeepAlive       = true
	defaultKeepAliveInterval     = 30 * time.Second
	defaultConnectionWriteTimeout = 10 * time.Second
	defaultStreamOpenTimeout     = 10 * time.Second
)

// Config holds the configuration options used to initialize a Yamux session
type Config struct {
	// AcceptBacklog is the maximum number of streams that the 
	// accept channel can buffer
	AcceptBacklog int

	// EnableKeepAlive will periodically send pings to keep connections alive
	EnableKeepAlive bool

	// KeepAliveInterval is the duration between keep-alive pings
	KeepAliveInterval time.Duration

	// ConnectionWriteTimeout is the amount of time a write to the connection
	// can block before timeout
	ConnectionWriteTimeout time.Duration

	// StreamOpenTimeout is the amount of time a stream open can block
	// before timeout
	StreamOpenTimeout time.Duration
}

// NewConfig creates a default configuration
func NewConfig() *Config {
	return &Config{
		AcceptBacklog:         defaultAcceptBacklog,
		EnableKeepAlive:       defaultEnableKeepAlive,
		KeepAliveInterval:     defaultKeepAliveInterval,
		ConnectionWriteTimeout: defaultConnectionWriteTimeout,
		StreamOpenTimeout:     defaultStreamOpenTimeout,
	}
}

// Session is a Yamux session
type Session struct {
	conn    io.ReadWriteCloser
	config  *Config
	
	// Stream handling
	streams    map[uint32]*Stream
	nextStreamID uint32
	streamLock sync.Mutex
	acceptCh   chan *Stream

	// Connection management
	isRemoteClient bool // Is this a server or client
	closed        bool
	closeLock     sync.Mutex
	
	// Reader loop
	readerShutdown chan struct{}
	
	// Writer loop
	writerLock     sync.Mutex
	writeCh       chan []byte
	writerShutdown chan struct{}
	
	// Keep alive
	keepaliveLock  sync.Mutex
	keepaliveTimer *time.Timer
}

// Server is used to initialize a server-side session
func Server(conn io.ReadWriteCloser, config *Config) (*Session, error) {
	if config == nil {
		config = NewConfig()
	}
	
	s := &Session{
		conn:           conn,
		config:         config,
		streams:        make(map[uint32]*Stream),
		nextStreamID:   1,
		acceptCh:       make(chan *Stream, config.AcceptBacklog),
		isRemoteClient: true,
		readerShutdown: make(chan struct{}),
		writeCh:        make(chan []byte, 16),
		writerShutdown: make(chan struct{}),
	}
	
	// Start the reader and writer
	go s.reader()
	go s.writer()
	
	// Start the keep-alive if enabled
	if config.EnableKeepAlive {
		go s.keepalive()
	}
	
	return s, nil
}

// Client is used to initialize a client-side session
func Client(conn io.ReadWriteCloser, config *Config) (*Session, error) {
	if config == nil {
		config = NewConfig()
	}
	
	s := &Session{
		conn:           conn,
		config:         config,
		streams:        make(map[uint32]*Stream),
		nextStreamID:   2, // Client uses even stream IDs
		acceptCh:       make(chan *Stream, config.AcceptBacklog),
		isRemoteClient: false,
		readerShutdown: make(chan struct{}),
		writeCh:        make(chan []byte, 16),
		writerShutdown: make(chan struct{}),
	}
	
	// Start the reader and writer
	go s.reader()
	go s.writer()
	
	// Start the keep-alive if enabled
	if config.EnableKeepAlive {
		go s.keepalive()
	}
	
	return s, nil
}

// IsClosed checks if the session is closed
func (s *Session) IsClosed() bool {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	return s.closed
}

// OpenStream creates a new stream
func (s *Session) OpenStream() (*Stream, error) {
	// Check if the session is closed
	if s.IsClosed() {
		return nil, fmt.Errorf("session closed")
	}
	
	// Get a stream ID
	s.streamLock.Lock()
	streamID := s.nextStreamID
	s.nextStreamID += 2 // Use odd/even IDs based on client/server
	stream := newStream(s, streamID)
	s.streams[streamID] = stream
	s.streamLock.Unlock()
	
	// Send a SYN message
	header := make([]byte, headerSize)
	header[0] = typeSYN
	header[1] = 0 // Flags
	binary.BigEndian.PutUint32(header[2:6], streamID)
	binary.BigEndian.PutUint32(header[6:10], 0) // Length is 0 for SYN
	
	// Write the header
	if err := s.write(header); err != nil {
		return nil, err
	}
	
	return stream, nil
}

// AcceptStream accepts a new stream
func (s *Session) AcceptStream() (*Stream, error) {
	if s.IsClosed() {
		return nil, fmt.Errorf("session closed")
	}
	
	select {
	case stream := <-s.acceptCh:
		return stream, nil
	case <-time.After(s.config.StreamOpenTimeout):
		return nil, fmt.Errorf("timeout waiting for new stream")
	}
}

// Close closes the session and all streams
func (s *Session) Close() error {
	s.closeLock.Lock()
	defer s.closeLock.Unlock()
	
	if s.closed {
		return nil
	}
	
	s.closed = true
	
	// Close all streams
	s.streamLock.Lock()
	for _, stream := range s.streams {
		stream.closed = true
		if stream.readBuf != nil {
			stream.readBuf.Close()
		}
	}
	s.streamLock.Unlock()
	
	// Stop the reader and writer
	close(s.readerShutdown)
	close(s.writerShutdown)
	
	// Close the underlying connection
	return s.conn.Close()
}

// reader is the main read loop
func (s *Session) reader() {
	for {
		select {
		case <-s.readerShutdown:
			return
		default:
			// Read the header
			header := make([]byte, headerSize)
			if _, err := io.ReadFull(s.conn, header); err != nil {
				if err != io.EOF {
					fmt.Printf("Error reading header: %v\n", err)
				}
				s.Close()
				return
			}
			
			// Decode the header
			msgType := header[0]
			flags := header[1]
			streamID := binary.BigEndian.Uint32(header[2:6])
			length := binary.BigEndian.Uint32(header[6:10])
			
			// Handle the message based on type
			switch msgType {
			case typeData:
				s.handleData(streamID, flags, length)
			case typeSYN:
				s.handleSYN(streamID)
			case typeFIN:
				s.handleFIN(streamID)
			case typeACK:
				s.handleACK(streamID)
			case typePING:
				s.handlePING(flags)
			case typePONG:
				// Nothing to do
			default:
				fmt.Printf("Unknown message type: %d\n", msgType)
			}
		}
	}
}

// handleData processes data frames
func (s *Session) handleData(streamID uint32, flags byte, length uint32) {
	// Get the stream
	s.streamLock.Lock()
	stream, ok := s.streams[streamID]
	s.streamLock.Unlock()
	
	if !ok {
		// Stream doesn't exist, discard the data
		if length > 0 {
			discard := make([]byte, length)
			io.ReadFull(s.conn, discard)
		}
		return
	}
	
	// Read the data
	if length > 0 {
		data := make([]byte, length)
		if _, err := io.ReadFull(s.conn, data); err != nil {
			fmt.Printf("Error reading data: %v\n", err)
			s.Close()
			return
		}
		
		// Give the data to the stream
		stream.readBuf.Write(data)
	}
}

// handleSYN processes stream creation
func (s *Session) handleSYN(streamID uint32) {
	s.streamLock.Lock()
	defer s.streamLock.Unlock()
	
	// Check if the stream already exists
	if _, ok := s.streams[streamID]; ok {
		fmt.Printf("Stream %d already exists\n", streamID)
		return
	}
	
	// Create the stream
	stream := newStream(s, streamID)
	s.streams[streamID] = stream
	
	// Send an ACK
	header := make([]byte, headerSize)
	header[0] = typeACK
	header[1] = 0 // Flags
	binary.BigEndian.PutUint32(header[2:6], streamID)
	binary.BigEndian.PutUint32(header[6:10], 0) // Length is 0 for ACK
	s.write(header)
	
	// Add to the accept channel
	select {
	case s.acceptCh <- stream:
	default:
		// Accept channel is full
		fmt.Println("Accept channel is full, dropping stream")
		delete(s.streams, streamID)
	}
}

// handleFIN processes stream closure
func (s *Session) handleFIN(streamID uint32) {
	s.streamLock.Lock()
	stream, ok := s.streams[streamID]
	s.streamLock.Unlock()
	
	if !ok {
		return
	}
	
	// Close the stream
	stream.closed = true
	stream.readBuf.Close()
}

// handleACK processes stream acknowledgment
func (s *Session) handleACK(streamID uint32) {
	// ACK is mainly for the SYN handshake, nothing to do here
}

// handlePING sends a PONG response
func (s *Session) handlePING(flags byte) {
	header := make([]byte, headerSize)
	header[0] = typePONG
	header[1] = flags
	header[2] = 0 // No stream ID for PING/PONG
	header[6] = 0 // No length for PING/PONG
	s.write(header)
}

// write queues a write to the writer
func (s *Session) write(data []byte) error {
	// Check if the session is closed
	if s.IsClosed() {
		return fmt.Errorf("session closed")
	}
	
	// Send the data to the writer
	select {
	case s.writeCh <- data:
		return nil
	case <-time.After(s.config.ConnectionWriteTimeout):
		return fmt.Errorf("write timeout")
	}
}

// writer is the main write loop
func (s *Session) writer() {
	for {
		select {
		case <-s.writerShutdown:
			return
		case data := <-s.writeCh:
			s.writerLock.Lock()
			_, err := s.conn.Write(data)
			s.writerLock.Unlock()
			
			if err != nil {
				fmt.Printf("Error writing: %v\n", err)
				s.Close()
				return
			}
		}
	}
}

// keepalive sends periodic PING messages
func (s *Session) keepalive() {
	for {
		select {
		case <-s.readerShutdown:
			return
		case <-time.After(s.config.KeepAliveInterval):
			// Send a PING
			header := make([]byte, headerSize)
			header[0] = typePING
			header[1] = 0 // Flags
			binary.BigEndian.PutUint32(header[2:6], 0) // No stream ID
			binary.BigEndian.PutUint32(header[6:10], 0) // No length
			
			if err := s.write(header); err != nil {
				fmt.Printf("Error sending keepalive: %v\n", err)
				s.Close()
				return
			}
		}
	}
}

// Stream is a logical stream within a session
type Stream struct {
	session *Session
	id      uint32
	closed  bool
	
	readBuf *buffer
}

// newStream creates a new stream
func newStream(session *Session, id uint32) *Stream {
	return &Stream{
		session: session,
		id:      id,
		readBuf: newBuffer(),
	}
}

// Read reads data from the stream
func (s *Stream) Read(p []byte) (int, error) {
	if s.closed {
		return 0, io.EOF
	}
	
	return s.readBuf.Read(p)
}

// Write writes data to the stream
func (s *Stream) Write(p []byte) (int, error) {
	if s.closed {
		return 0, fmt.Errorf("stream closed")
	}
	
	// Check if there's data to write
	if len(p) == 0 {
		return 0, nil
	}
	
	// Create the header
	header := make([]byte, headerSize)
	header[0] = typeData
	header[1] = 0 // Flags
	binary.BigEndian.PutUint32(header[2:6], s.id)
	binary.BigEndian.PutUint32(header[6:10], uint32(len(p)))
	
	// Write the header and data
	data := append(header, p...)
	err := s.session.write(data)
	if err != nil {
		return 0, err
	}
	
	return len(p), nil
}

// Close closes the stream
func (s *Stream) Close() error {
	if s.closed {
		return nil
	}
	
	s.closed = true
	
	// Send a FIN message
	header := make([]byte, headerSize)
	header[0] = typeFIN
	header[1] = 0 // Flags
	binary.BigEndian.PutUint32(header[2:6], s.id)
	binary.BigEndian.PutUint32(header[6:10], 0) // Length is 0 for FIN
	
	return s.session.write(header)
}

// buffer is a bytes buffer for the stream
type buffer struct {
	buf   []byte
	rd, wr int
	
	closed bool
	mutex  sync.Mutex
	signal chan struct{}
}

// newBuffer creates a new buffer
func newBuffer() *buffer {
	return &buffer{
		buf:    make([]byte, 0, 64),
		signal: make(chan struct{}, 1),
	}
}

// Read reads data from the buffer
func (b *buffer) Read(p []byte) (int, error) {
	b.mutex.Lock()
	
	for b.rd >= b.wr {
		// Buffer is empty
		if b.closed {
			b.mutex.Unlock()
			return 0, io.EOF
		}
		
		// Wait for more data
		b.mutex.Unlock()
		<-b.signal
		b.mutex.Lock()
	}
	
	// Read the data
	n := copy(p, b.buf[b.rd:b.wr])
	b.rd += n
	
	// Compact the buffer if it's getting full
	if b.rd == b.wr {
		b.rd = 0
		b.wr = 0
	}
	
	b.mutex.Unlock()
	return n, nil
}

// Write writes data to the buffer
func (b *buffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	if b.closed {
		return 0, fmt.Errorf("buffer closed")
	}
	
	// Grow the buffer if needed
	if b.wr+len(p) > len(b.buf) {
		// Double the buffer size until it can fit the data
		newSize := cap(b.buf) * 2
		for newSize < b.wr+len(p) {
			newSize *= 2
		}
		
		newBuf := make([]byte, newSize)
		copy(newBuf, b.buf[:b.wr])
		b.buf = newBuf
	}
	
	// Write the data
	n := copy(b.buf[b.wr:], p)
	b.wr += n
	
	// Signal that data is available
	select {
	case b.signal <- struct{}{}:
	default:
	}
	
	return n, nil
}

// Close closes the buffer
func (b *buffer) Close() error {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	
	if b.closed {
		return nil
	}
	
	b.closed = true
	
	// Signal to unblock any waiting readers
	select {
	case b.signal <- struct{}{}:
	default:
	}
	
	return nil
} 