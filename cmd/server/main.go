// Main entry point for the File Sharing Utility server
package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"file-sharing-utility/internal/httpserver"
	"file-sharing-utility/internal/socks"
)

// Configuration options
type Config struct {
	ListenAddr      string
	SocksAddr       string
	EnableSocks     bool
	EnableHttp      bool
	XorKey          string
	DownloadPath    string
	UploadPath      string
}

func main() {
	// Parse command line flags
	config := parseFlags()

	// Ensure download and upload directories exist
	ensureDirectories(config)

	// Set up signal handling for graceful shutdown
	setupSignalHandling()

	// Start the HTTP server if enabled
	if config.EnableHttp {
		startHTTPServer(config)
	}

	// Start the SOCKS5 proxy if enabled
	if config.EnableSocks {
		startSocksServer(config)
	}

	// Block forever (or until signal is received)
	select {}
}

// parseFlags parses command line flags
func parseFlags() *Config {
	config := &Config{}

	flag.StringVar(&config.ListenAddr, "listen", "127.0.0.1:8080", "Address to listen on")
	flag.StringVar(&config.SocksAddr, "socks", "127.0.0.1:1080", "SOCKS5 proxy address")
	flag.BoolVar(&config.EnableSocks, "enable-socks", true, "Enable SOCKS5 proxy")
	flag.BoolVar(&config.EnableHttp, "enable-http", true, "Enable HTTP server")
	flag.StringVar(&config.XorKey, "xor-key", "", "XOR key for encoding/decoding")
	flag.StringVar(&config.DownloadPath, "download-path", "./downloads", "Path to download files")
	flag.StringVar(&config.UploadPath, "upload-path", "./uploads", "Path to upload files")
	
	flag.Parse()
	
	return config
}

// ensureDirectories ensures that the download and upload directories exist
func ensureDirectories(config *Config) {
	os.MkdirAll(config.DownloadPath, 0755)
	os.MkdirAll(config.UploadPath, 0755)
}

// setupSignalHandling sets up signal handling for graceful shutdown
func setupSignalHandling() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		log.Println("Received signal, shutting down...")
		os.Exit(0)
	}()
}

// startHTTPServer starts the HTTP server
func startHTTPServer(config *Config) {
	server := httpserver.NewServer(
		config.DownloadPath,
		config.UploadPath,
		config.XorKey,
	)
	
	// Setup yamux support
	server.SetupYamux()
	
	// Start the server in a goroutine
	go func() {
		log.Printf("Starting HTTP server on %s", config.ListenAddr)
		err := server.ListenAndServe(config.ListenAddr)
		if err != nil {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()
}

// startSocksServer starts the SOCKS5 proxy server
func startSocksServer(config *Config) {
	server, err := socks.NewServer(config.SocksAddr, config.XorKey)
	if err != nil {
		log.Fatalf("Failed to create SOCKS5 server: %v", err)
	}
	
	// Start the server in a goroutine
	server.StartAsync()
} 