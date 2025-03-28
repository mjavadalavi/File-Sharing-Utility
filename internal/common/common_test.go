package common

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetInfo(t *testing.T) {
	info := GetInfo()
	
	// Check if basic fields are populated
	if info.Hostname == "" {
		t.Error("Hostname is empty")
	}
	
	if info.OS == "" {
		t.Error("OS is empty")
	}
	
	if info.Version == "" {
		t.Error("Version is empty")
	}
	
	if info.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
	
	if info.NumCPU <= 0 {
		t.Errorf("NumCPU should be positive, got %d", info.NumCPU)
	}
}

func TestInfoString(t *testing.T) {
	info := GetInfo()
	strInfo := info.String()
	
	// Check if all the expected fields are in the string representation
	expectedFields := []string{
		"Hostname:",
		"OS:",
		"Version:",
		"Go Version:",
		"NumCPU:",
		"Uptime:",
		"Uploads:",
		"Downloads:",
	}
	
	for _, field := range expectedFields {
		if !strings.Contains(strInfo, field) {
			t.Errorf("Expected to find '%s' in info string, but it's missing", field)
		}
	}
}

func TestWriteBlob(t *testing.T) {
	// Test data
	testData := []byte("test data for WriteBlob")
	
	// Source and destination
	src := bytes.NewReader(testData)
	dst := &bytes.Buffer{}
	
	// Write the blob
	n, err := WriteBlob(dst, src)
	if err != nil {
		t.Fatalf("WriteBlob failed: %v", err)
	}
	
	// Check the number of bytes written
	if n != int64(len(testData)) {
		t.Errorf("Expected to write %d bytes, but wrote %d", len(testData), n)
	}
	
	// Check the content
	if !bytes.Equal(dst.Bytes(), testData) {
		t.Errorf("Content mismatch. Got %v, want %v", dst.Bytes(), testData)
	}
}

func TestWriteBlobWithError(t *testing.T) {
	// Test with an error source
	errorReader := &errorReadWriter{readErr: io.ErrUnexpectedEOF}
	dst := &bytes.Buffer{}
	
	_, err := WriteBlob(dst, errorReader)
	if err != io.ErrUnexpectedEOF {
		t.Errorf("Expected ErrUnexpectedEOF, got %v", err)
	}
	
	// Test with an error destination
	src := bytes.NewReader([]byte("test data"))
	errorWriter := &errorReadWriter{writeErr: io.ErrShortWrite}
	
	_, err = WriteBlob(errorWriter, src)
	if err != io.ErrShortWrite {
		t.Errorf("Expected ErrShortWrite, got %v", err)
	}
}

// Setup a temporary file for testing file operations
func setupTempFile(t *testing.T, data []byte) (string, func()) {
	tmpDir, err := os.MkdirTemp("", "common-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	
	filePath := filepath.Join(tmpDir, "test-file.txt")
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		os.RemoveAll(tmpDir)
		t.Fatalf("Failed to write test file: %v", err)
	}
	
	cleanup := func() {
		os.RemoveAll(tmpDir)
	}
	
	return filePath, cleanup
}

func TestReadBlob(t *testing.T) {
	// Test data
	testData := []byte("test data for ReadBlob")
	
	// Setup temp file
	filePath, cleanup := setupTempFile(t, testData)
	defer cleanup()
	
	// Read the blob
	data, err := ReadBlob(filePath)
	if err != nil {
		t.Fatalf("ReadBlob failed: %v", err)
	}
	
	// Check the content
	if !bytes.Equal(data, testData) {
		t.Errorf("Content mismatch. Got %v, want %v", data, testData)
	}
	
	// Test reading non-existent file
	_, err = ReadBlob("non-existent-file.txt")
	if !os.IsNotExist(err) {
		t.Errorf("Expected os.ErrNotExist, got %v", err)
	}
}

func TestSaveBlob(t *testing.T) {
	// Test data
	testData := []byte("test data for SaveBlob")
	
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "common-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	
	filePath := filepath.Join(tmpDir, "save-test.txt")
	
	// Save the blob
	err = SaveBlob(filePath, testData)
	if err != nil {
		t.Fatalf("SaveBlob failed: %v", err)
	}
	
	// Read the file back to verify
	savedData, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read saved file: %v", err)
	}
	
	if !bytes.Equal(savedData, testData) {
		t.Errorf("Content mismatch. Got %v, want %v", savedData, testData)
	}
}

func TestFileExists(t *testing.T) {
	// Setup temp file
	testData := []byte("test data for FileExists")
	filePath, cleanup := setupTempFile(t, testData)
	defer cleanup()
	
	// Check existing file
	if !FileExists(filePath) {
		t.Errorf("FileExists returned false for existing file %s", filePath)
	}
	
	// Check non-existent file
	if FileExists("non-existent-file.txt") {
		t.Errorf("FileExists returned true for non-existent file")
	}
}

func TestAppendToFile(t *testing.T) {
	// Test data
	initialData := []byte("initial data\n")
	appendData := []byte("appended data")
	
	// Setup temp file
	filePath, cleanup := setupTempFile(t, initialData)
	defer cleanup()
	
	// Append to the file
	err := AppendToFile(filePath, appendData)
	if err != nil {
		t.Fatalf("AppendToFile failed: %v", err)
	}
	
	// Read the file back to verify
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file after append: %v", err)
	}
	
	expectedData := append(initialData, appendData...)
	if !bytes.Equal(data, expectedData) {
		t.Errorf("Content mismatch. Got %v, want %v", data, expectedData)
	}
	
	// Test appending to non-existent file (should create it)
	nonExistentPath := filepath.Join(filepath.Dir(filePath), "non-existent.txt")
	err = AppendToFile(nonExistentPath, appendData)
	if err != nil {
		t.Fatalf("AppendToFile failed for non-existent file: %v", err)
	}
	
	// Verify the file was created with the correct content
	data, err = os.ReadFile(nonExistentPath)
	if err != nil {
		t.Fatalf("Failed to read newly created file: %v", err)
	}
	
	if !bytes.Equal(data, appendData) {
		t.Errorf("Content mismatch for new file. Got %v, want %v", data, appendData)
	}
}

// errorReadWriter is a helper type for testing error cases
type errorReadWriter struct {
	readErr  error
	writeErr error
}

func (e *errorReadWriter) Read(p []byte) (int, error) {
	if e.readErr != nil {
		return 0, e.readErr
	}
	return len(p), nil
}

func (e *errorReadWriter) Write(p []byte) (int, error) {
	if e.writeErr != nil {
		return 0, e.writeErr
	}
	return len(p), nil
} 