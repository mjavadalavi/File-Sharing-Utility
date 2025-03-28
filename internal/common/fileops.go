package common

import (
	"io"
	"os"
)

// WriteBlob copies data from a reader to a writer and returns the number of bytes copied
func WriteBlob(dst io.Writer, src io.Reader) (int64, error) {
	return io.Copy(dst, src)
}

// ReadBlob reads data from a file and returns it as a byte slice
func ReadBlob(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// SaveBlob writes data to a file
func SaveBlob(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// AppendToFile appends data to a file
func AppendToFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()
	
	_, err = f.Write(data)
	return err
} 