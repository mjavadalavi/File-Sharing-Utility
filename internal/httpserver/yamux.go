package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/hashicorp/yamux"

	"file-sharing-utility/internal/common"
	"file-sharing-utility/internal/xorrw"
)

// SetupYamux configures yamux support for the HTTP server
func (s *Server) SetupYamux() {
	s.mux.HandleFunc("/yamux", s.handleYamux)
}

// handleYamux handles yamux connection requests
func (s *Server) handleYamux(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received yamux connection request from %s", r.RemoteAddr)
	
	// Hijack the connection
	hj, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Webserver doesn't support hijacking", http.StatusInternalServerError)
		return
	}
	
	conn, bufrw, err := hj.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	
	// Send a 101 Switching Protocols response
	bufrw.WriteString("HTTP/1.1 101 Switching Protocols\r\n")
	bufrw.WriteString("Upgrade: yamux\r\n")
	bufrw.WriteString("Connection: Upgrade\r\n")
	bufrw.WriteString("\r\n")
	bufrw.Flush()
	
	// Apply XOR encoding if a key is provided
	var rwConn io.ReadWriteCloser = conn
	if s.xorKey != "" {
		rwConn = xorrw.NewXorReaderWriter(conn, []byte(s.xorKey))
	}
	
	// Create yamux server session
	config := yamux.DefaultConfig()
	config.EnableKeepAlive = true
	config.KeepAliveInterval = 30 * time.Second
	config.ConnectionWriteTimeout = 10 * time.Second
	
	session, err := yamux.Server(rwConn, config)
	if err != nil {
		log.Printf("Failed creating yamux server: %v", err)
		conn.Close()
		return
	}
	
	// Handle the session in a goroutine
	go s.handleYamuxSession(session)
}

// handleYamuxSession manages a yamux session and its streams
func (s *Server) handleYamuxSession(session *yamux.Session) {
	defer session.Close()
	
	log.Printf("Started yamux session")
	
	for {
		// Accept a new stream
		stream, err := session.AcceptStream()
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed accepting yamux connection: %v", err)
			}
			break
		}
		
		// Handle the stream in a goroutine
		go s.handleYamuxStream(stream)
	}
	
	log.Printf("Yamux session closed")
}

// handleYamuxStream processes commands sent over a yamux stream
func (s *Server) handleYamuxStream(stream *yamux.Stream) {
	defer stream.Close()
	
	log.Printf("Accepted yamux stream %d", stream.StreamID())
	
	// Create a buffered reader for the stream
	reader := newCommandReader(stream)
	
	for {
		// Read a command
		cmd, err := reader.readCommand()
		if err != nil {
			if err != io.EOF {
				log.Printf("Failed to read command: %v", err)
			}
			break
		}
		
		// Process the command
		response := s.processCommand(cmd)
		
		// Send the response
		if _, err := stream.Write([]byte(response)); err != nil {
			log.Printf("Failed to send reply: %v", err)
			break
		}
	}
}

// Command represents a client command
type Command struct {
	Type    string            `json:"type"`
	Path    string            `json:"path,omitempty"`
	Content []byte            `json:"content,omitempty"`
	Params  map[string]string `json:"params,omitempty"`
}

// commandReader reads commands from a reader
type commandReader struct {
	r io.Reader
}

// newCommandReader creates a new commandReader
func newCommandReader(r io.Reader) *commandReader {
	return &commandReader{r: r}
}

// readCommand reads and parses a command
func (cr *commandReader) readCommand() (*Command, error) {
	// Read the command length (4 bytes)
	var length [4]byte
	if _, err := io.ReadFull(cr.r, length[:]); err != nil {
		return nil, err
	}
	
	// Convert to integer (assuming little-endian)
	cmdLen := int(length[0]) | int(length[1])<<8 | int(length[2])<<16 | int(length[3])<<24
	
	// Read the command data
	cmdData := make([]byte, cmdLen)
	if _, err := io.ReadFull(cr.r, cmdData); err != nil {
		return nil, err
	}
	
	// Parse the command
	var cmd Command
	if err := json.Unmarshal(cmdData, &cmd); err != nil {
		return nil, err
	}
	
	return &cmd, nil
}

// processCommand handles a command and returns a response
func (s *Server) processCommand(cmd *Command) string {
	switch cmd.Type {
	case "list":
		return s.handleListCommand(cmd)
	case "upload":
		return s.handleUploadCommand(cmd)
	case "download":
		return s.handleDownloadCommand(cmd)
	case "delete":
		return s.handleDeleteCommand(cmd)
	case "info":
		return s.handleInfoCommand()
	default:
		return "Unsupported command: " + cmd.Type
	}
}

// handleListCommand lists files in a directory
func (s *Server) handleListCommand(cmd *Command) string {
	dir := s.downloadPath
	if cmd.Path != "" {
		// Prevent directory traversal
		cleanPath := filepath.Clean(cmd.Path)
		if strings.Contains(cleanPath, "..") {
			return "Error: Invalid path"
		}
		dir = filepath.Join(s.downloadPath, cleanPath)
	}
	
	// Read the directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "Error reading directory: " + err.Error()
	}
	
	// Build the response
	var result strings.Builder
	for _, entry := range entries {
		if entry.IsDir() {
			result.WriteString("d ")
		} else {
			result.WriteString("f ")
		}
		
		info, err := entry.Info()
		if err == nil {
			result.WriteString(info.ModTime().Format("2006-01-02 15:04:05") + " ")
			result.WriteString(formatSize(info.Size()) + " ")
		}
		
		result.WriteString(entry.Name() + "\n")
	}
	
	return result.String()
}

// handleUploadCommand stores uploaded data
func (s *Server) handleUploadCommand(cmd *Command) string {
	if cmd.Path == "" {
		return "Error: Path not specified"
	}
	
	// Prevent directory traversal
	cleanPath := filepath.Clean(cmd.Path)
	if strings.Contains(cleanPath, "..") {
		return "Error: Invalid path"
	}
	
	// Create the target file
	targetPath := filepath.Join(s.uploadPath, cleanPath)
	
	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return "Error creating directory: " + err.Error()
	}
	
	// Write the content
	if err := os.WriteFile(targetPath, cmd.Content, 0644); err != nil {
		return "Error writing file: " + err.Error()
	}
	
	return "File uploaded successfully"
}

// handleDownloadCommand retrieves file data
func (s *Server) handleDownloadCommand(cmd *Command) string {
	if cmd.Path == "" {
		return "Error: Path not specified"
	}
	
	// Prevent directory traversal
	cleanPath := filepath.Clean(cmd.Path)
	if strings.Contains(cleanPath, "..") {
		return "Error: Invalid path"
	}
	
	// Read the file
	targetPath := filepath.Join(s.downloadPath, cleanPath)
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "Error reading file: " + err.Error()
	}
	
	// Return it as a command response
	response := &Command{
		Type:    "file_data",
		Path:    cmd.Path,
		Content: data,
	}
	
	jsonData, err := json.Marshal(response)
	if err != nil {
		return "Error serializing data: " + err.Error()
	}
	
	return string(jsonData)
}

// handleDeleteCommand deletes a file
func (s *Server) handleDeleteCommand(cmd *Command) string {
	if cmd.Path == "" {
		return "Error: Path not specified"
	}
	
	// Prevent directory traversal
	cleanPath := filepath.Clean(cmd.Path)
	if strings.Contains(cleanPath, "..") {
		return "Error: Invalid path"
	}
	
	// Determine which base path to use
	var basePath string
	if cmd.Params != nil && cmd.Params["location"] == "upload" {
		basePath = s.uploadPath
	} else {
		basePath = s.downloadPath
	}
	
	// Delete the file
	targetPath := filepath.Join(basePath, cleanPath)
	if err := os.Remove(targetPath); err != nil {
		return "Error deleting file: " + err.Error()
	}
	
	return "File deleted successfully"
}

// handleInfoCommand returns system information
func (s *Server) handleInfoCommand() string {
	info := common.GetInfo()
	return info.String()
}

// formatSize formats a file size in a human-readable format
func formatSize(size int64) string {
	const (
		_        = iota
		KB int64 = 1 << (10 * iota)
		MB
		GB
	)
	
	switch {
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/float64(GB))
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/float64(MB))
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/float64(KB))
	default:
		return fmt.Sprintf("%d B", size)
	}
} 