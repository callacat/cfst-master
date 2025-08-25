# The binary name
BINARY=cfst-client

.PHONY: build build-linux-amd64 build-linux-arm64 build-windows-amd64 build-windows-386 clean

# Default build target
build:
	@echo "Building for the current OS and architecture..."
	go build -o $(BINARY) ./cmd/main.go

# Build for Linux AMD64
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 ./cmd/main.go

# Build for Linux ARM64
build-linux-arm64:
	@echo "Building for Linux ARM64..."
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/$(BINARY)-linux-arm64 ./cmd/main.go

# Build for Windows AMD64 (x64)
build-windows-amd64:
	@echo "Building for Windows AMD64..."
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe ./cmd/main.go

# Build for Windows 386 (x86)
build-windows-386:
	@echo "Building for Windows 386..."
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -o bin/$(BINARY)-windows-386.exe ./cmd/main.go

# Clean up build artifacts
clean:
	@echo "Cleaning up..."
	rm -f $(BINARY)
	rm -rf bin/

