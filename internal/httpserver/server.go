// Package httpserver provides HTTP server functionality
package httpserver

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"file-sharing-utility/internal/common"
	"file-sharing-utility/internal/xorrw"
)

// Server represents the HTTP server
type Server struct {
	mux          *Mux
	downloadPath string
	uploadPath   string
	xorKey       string
}

// NewServer creates a new HTTP server
func NewServer(downloadPath, uploadPath, xorKey string) *Server {
	server := &Server{
		mux:          NewMux(),
		downloadPath: downloadPath,
		uploadPath:   uploadPath,
		xorKey:       xorKey,
	}
	
	// Set up HTTP routes
	server.setupRoutes()
	
	return server
}

// ListenAndServe starts the HTTP server
func (s *Server) ListenAndServe(addr string) error {
	log.Printf("Starting HTTP server on %s", addr)
	return http.ListenAndServe(addr, s.mux)
}

// setupRoutes configures the HTTP routes
func (s *Server) setupRoutes() {
	// Handle file uploads
	s.mux.HandleFunc("/upload", s.handleUpload)
	
	// Handle file downloads
	s.mux.HandleFunc("/download", s.handleDownload)
	
	// Simple status endpoint
	s.mux.HandleFunc("/status", s.handleStatus)
}

// handleUpload handles file upload requests
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Failed to get file from request", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create the target file
	targetPath := filepath.Join(s.uploadPath, header.Filename)
	target, err := os.Create(targetPath)
	if err != nil {
		http.Error(w, "Failed to create target file", http.StatusInternalServerError)
		return
	}
	defer target.Close()

	// Apply XOR encoding if a key is provided
	var writer io.Writer = target
	if s.xorKey != "" {
		xorWriter := xorrw.NewXorReaderWriter(target, []byte(s.xorKey))
		defer xorWriter.Close()
		writer = xorWriter
	}

	// Copy the file contents
	_, err = common.WriteBlob(writer, file)
	if err != nil {
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("File uploaded successfully"))
}

// handleDownload handles file download requests
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	filename := r.URL.Query().Get("file")
	if filename == "" {
		http.Error(w, "Filename not provided", http.StatusBadRequest)
		return
	}

	// Prevent directory traversal
	if filepath.IsAbs(filename) || filepath.Clean(filename) != filename {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(s.downloadPath, filename)
	file, err := os.Open(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	defer file.Close()

	// Get file info for Content-Length header
	info, err := file.Stat()
	if err != nil {
		http.Error(w, "Failed to get file info", http.StatusInternalServerError)
		return
	}

	// Set response headers
	w.Header().Set("Content-Disposition", "attachment; filename="+filename)
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.FormatInt(info.Size(), 10))

	// Apply XOR decoding if a key is provided
	var reader io.Reader = file
	if s.xorKey != "" {
		xorReader := xorrw.NewXorReaderWriter(file, []byte(s.xorKey))
		defer xorReader.Close()
		reader = xorReader
	}

	// Copy the file to the response
	_, err = common.WriteBlob(w, reader)
	if err != nil {
		log.Printf("Error downloading file: %v", err)
	}
}

// handleStatus returns system information
func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	info := common.GetInfo()
	w.Write([]byte(info.String()))
}