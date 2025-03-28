# FilePhantom

![Version](https://img.shields.io/badge/version-1.0.0-blue.svg)
![License](https://img.shields.io/badge/license-MIT-green.svg)

A Go-based file sharing utility with HTTP server and SOCKS5 proxy capabilities.

## Features

- **HTTP Server** - Handles file uploads and downloads via HTTP endpoints
- **SOCKS5 Proxy** - Provides a SOCKS5 proxy for redirecting traffic
- **Yamux Multiplexing** - Supports multiple connections over a single TCP connection
- **XOR Encoding/Decoding** - Offers simple obfuscation for transferred data
- **File Management** - Supports uploading, downloading, listing, and deleting files

## Project Structure

```
.
├── api/              # API definitions and documentation (OpenAPI/Swagger)
├── cmd/              # Application entry points
│   └── server/       # The main server application
├── configs/          # Configuration files and templates
├── docs/             # Project documentation
├── internal/         # Private application code
│   ├── common/       # Common utilities and shared code
│   ├── httpserver/   # HTTP server implementation
│   ├── socks/        # SOCKS5 proxy implementation
│   └── xorrw/        # XOR reader/writer implementation
└── pkg/              # Public libraries that can be used by external applications
    ├── fileops/      # File operation utilities
    └── yamux/        # Yamux session handling
```

## Requirements

- Go 1.19 or higher

## Dependencies

- `github.com/armon/go-socks5` - SOCKS5 proxy implementation
- `github.com/hashicorp/yamux` - Multiplexing library for Go

## Building and Running

### Installation

```bash
# Clone the repository
git clone https://github.com/your-username/file-sharing-utility.git
cd file-sharing-utility

# Install dependencies
go mod tidy
```

### Building

```bash
# Build using make
make build

# Or using go command
go build -o bin/file-sharing-utility ./cmd/server
```

### Running

```bash
# Run using make
make run

# Or run the binary directly
./bin/file-sharing-utility -listen :8080 -socks :1080 -xor-key "secretkey"
```

### Building for Multiple Platforms

```bash
make build-all
```

## Using Docker

### Building the Image

```bash
docker build -t file-sharing-utility .
```

### Running with Docker

```bash
docker run -p 8080:8080 -p 1080:1080 \
  -e XOR_KEY=your_secret_key \
  -v $(pwd)/data:/app/data \
  file-sharing-utility
```

### Running with Docker Compose

```bash
docker-compose up -d
```

## Command-Line Options

```
-listen string
    Address to listen on (default "127.0.0.1:8080")
-socks string
    SOCKS5 proxy address (default "127.0.0.1:1080")
-enable-socks
    Enable SOCKS5 proxy (default true)
-enable-http
    Enable HTTP server (default true)
-xor-key string
    XOR key for encoding/decoding
-download-path string
    Path to download files (default "./downloads")
-upload-path string
    Path to upload files (default "./uploads")
```

## HTTP API Endpoints

### File Upload
```
POST /upload
```
Use multipart form to upload a file, form field should be named `file`.

### File Download
```
GET /download?file=filename
```
Download a file with the specified name.

### Server Status
```
GET /status
```
Get server information including hostname, OS, versions, and statistics.

### Yamux Connection
```
GET /yamux
```
Establish a yamux connection for multiplexed commands.

## Yamux Commands

When a yamux connection is established, you can send the following commands:

- `list` - List files in a directory
- `upload` - Upload files
- `download` - Download files
- `delete` - Delete files
- `info` - Get system information

## Security Considerations

This application has several security considerations:

1. The XOR encoding is a very weak form of obfuscation, not encryption
2. There is no authentication mechanism
3. Path traversal protection is minimal
4. No TLS/HTTPS support by default

## Contributing

Contributions are welcome! Please submit a pull request or open an issue for any problems or suggestions.

## License

This project is licensed under the MIT License. 
