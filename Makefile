.PHONY: build clean run test

# Build variables
BINARY_NAME=file-sharing-utility
GO=go
GOFMT=gofmt
GOGET=$(GO) get
GOBUILD=$(GO) build
GOCLEAN=$(GO) clean
GOTEST=$(GO) test
GOMOD=$(GO) mod
MKDIR=mkdir -p

# Output directories
BIN_DIR=bin
BUILD_DIR=build

# Compilation flags
LDFLAGS=-ldflags "-s -w"

all: clean build

# Build the application
build:
	$(MKDIR) $(BIN_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/server

# Clean the build artifacts
clean:
	$(GOCLEAN)
	rm -rf $(BIN_DIR)
	rm -rf $(BUILD_DIR)

# Run the application
run: build
	./$(BIN_DIR)/$(BINARY_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Install dependencies
deps:
	$(GOMOD) tidy

# Build for multiple platforms
build-all: clean
	$(MKDIR) $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd/server
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd/server

# Format the code
fmt:
	$(GOFMT) -s -w . 