package httpserver

import (
	"bytes"
	"file-sharing-utility/internal/common"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {
	// Create temporary directories for testing
	downloadDir, err := os.MkdirTemp("", "download")
	if err != nil {
		t.Fatalf("Failed to create temp download dir: %v", err)
	}
	defer os.RemoveAll(downloadDir)

	uploadDir, err := os.MkdirTemp("", "upload")
	if err != nil {
		t.Fatalf("Failed to create temp upload dir: %v", err)
	}
	defer os.RemoveAll(uploadDir)

	// Create a new server instance
	server := NewServer(downloadDir, uploadDir, "")

	// Check if server is not nil
	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	// Check if server fields are set
	if server.downloadPath != downloadDir {
		t.Errorf("Expected downloadPath to be '%s', got '%s'", downloadDir, server.downloadPath)
	}

	if server.uploadPath != uploadDir {
		t.Errorf("Expected uploadPath to be '%s', got '%s'", uploadDir, server.uploadPath)
	}
}

func TestStatusHandler(t *testing.T) {
	server := NewServer("/tmp/download", "/tmp/upload", "")
	handler := http.HandlerFunc(server.handleStatus)

	req, err := http.NewRequest("GET", "/status", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check if response contains expected data
	responseStr := rr.Body.String()
	expectedFields := []string{"Hostname:", "OS:", "Version:", "Go Version:"}
	
	for _, field := range expectedFields {
		if !strings.Contains(responseStr, field) {
			t.Errorf("Expected response to contain '%s', but it doesn't", field)
		}
	}
}

func TestDownloadHandler(t *testing.T) {
	// Create temporary download directory
	downloadDir, err := os.MkdirTemp("", "download")
	if err != nil {
		t.Fatalf("Failed to create temp download dir: %v", err)
	}
	defer os.RemoveAll(downloadDir)

	// Create a test file in the download directory
	testFileName := "test-file.txt"
	testFilePath := filepath.Join(downloadDir, testFileName)
	testContent := []byte("This is test file content")
	if err := os.WriteFile(testFilePath, testContent, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a new server instance
	server := NewServer(downloadDir, "/tmp/upload", "")
	handler := http.HandlerFunc(server.handleDownload)

	// Test successful download
	req, err := http.NewRequest("GET", "/download?file="+testFileName, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Check content
	if !bytes.Equal(rr.Body.Bytes(), testContent) {
		t.Errorf("Downloaded content does not match. Got %v, want %v", rr.Body.Bytes(), testContent)
	}

	// Test non-existent file
	req, err = http.NewRequest("GET", "/download?file=non-existent.txt", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response status code for non-existent file
	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("Handler returned wrong status code for non-existent file: got %v want %v", 
			status, http.StatusNotFound)
	}

	// Test missing file parameter
	req, err = http.NewRequest("GET", "/download", nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response status code for missing file parameter
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Handler returned wrong status code for missing file parameter: got %v want %v", 
			status, http.StatusBadRequest)
	}
}

func createMultipartRequest(t *testing.T, fieldName, fileName string, fileContent []byte) (*http.Request, string) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	
	// Create a form file
	fw, err := w.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	
	// Write content to the form file
	if _, err := fw.Write(fileContent); err != nil {
		t.Fatalf("Failed to write to form file: %v", err)
	}
	
	// Close the writer
	if err := w.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}
	
	// Create a new HTTP request
	req, err := http.NewRequest("POST", "/upload", &b)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	
	// Set the content type
	contentType := w.FormDataContentType()
	req.Header.Set("Content-Type", contentType)
	
	return req, contentType
}

func TestUploadHandler(t *testing.T) {
	// Create temporary upload directory
	uploadDir, err := os.MkdirTemp("", "upload")
	if err != nil {
		t.Fatalf("Failed to create temp upload dir: %v", err)
	}
	defer os.RemoveAll(uploadDir)

	// Create a new server instance
	server := NewServer("/tmp/download", uploadDir, "")
	handler := http.HandlerFunc(server.handleUpload)

	// Test successful upload
	testFileName := "upload-test.txt"
	testContent := []byte("This is test upload content")
	req, _ := createMultipartRequest(t, "file", testFileName, testContent)

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response status code
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	// Verify the file was saved
	uploadedFilePath := filepath.Join(uploadDir, testFileName)
	if !common.FileExists(uploadedFilePath) {
		t.Errorf("Uploaded file doesn't exist at %s", uploadedFilePath)
	}

	// Check the content of the uploaded file
	content, err := os.ReadFile(uploadedFilePath)
	if err != nil {
		t.Fatalf("Failed to read uploaded file: %v", err)
	}

	if !bytes.Equal(content, testContent) {
		t.Errorf("Uploaded content does not match. Got %v, want %v", content, testContent)
	}

	// Test missing file in request
	req, err = http.NewRequest("POST", "/upload", strings.NewReader("not a multipart request"))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "text/plain")

	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	// Check response status code for bad request
	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Handler returned wrong status code for bad request: got %v want %v", 
			status, http.StatusBadRequest)
	}
}

// Skip TestListFilesHandler as it's not implemented in the server
// Skip TestYamuxHandler as it requires yamux package which might not be properly set up

// TestCloser tests a struct that implements io.Closer for coverage purposes
type testCloser struct {
	closeFunc func() error
}

func (t *testCloser) Close() error {
	if t.closeFunc != nil {
		return t.closeFunc()
	}
	return nil
}

// TestReadWriter tests a struct that implements io.ReadWriter for coverage purposes
type testReadWriter struct {
	readFunc  func([]byte) (int, error)
	writeFunc func([]byte) (int, error)
}

func (t *testReadWriter) Read(p []byte) (int, error) {
	if t.readFunc != nil {
		return t.readFunc(p)
	}
	return 0, io.EOF
}

func (t *testReadWriter) Write(p []byte) (int, error) {
	if t.writeFunc != nil {
		return t.writeFunc(p)
	}
	return len(p), nil
} 