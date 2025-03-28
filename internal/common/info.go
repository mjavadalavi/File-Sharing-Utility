// Package common provides common utilities and shared code
package common

import (
	"fmt"
	"os"
	"runtime"
	"time"
)

// Info holds system and application information
type Info struct {
	Hostname      string
	OS            string
	Version       string
	GoVersion     string
	NumCPU        int
	StartTime     time.Time
	UploadCount   int
	DownloadCount int
}

// GetInfo returns system and application information
func GetInfo() *Info {
	hostname, _ := os.Hostname()
	
	info := &Info{
		Hostname:    hostname,
		OS:          runtime.GOOS,
		Version:     "1.0.0", // Assumed version
		GoVersion:   runtime.Version(),
		NumCPU:      runtime.NumCPU(),
		StartTime:   time.Now(), // This should be set at application startup
	}
	
	return info
}

// String returns a string representation of the Info struct
func (i *Info) String() string {
	uptime := time.Since(i.StartTime)
	
	return fmt.Sprintf(
		"Server Information:\n"+
		"Hostname: %s\n"+
		"OS: %s\n"+
		"Version: %s\n"+
		"Go Version: %s\n"+
		"NumCPU: %d\n"+
		"Uptime: %s\n"+
		"Uploads: %d\n"+
		"Downloads: %d\n",
		i.Hostname,
		i.OS,
		i.Version,
		i.GoVersion,
		i.NumCPU,
		uptime,
		i.UploadCount,
		i.DownloadCount,
	)
} 