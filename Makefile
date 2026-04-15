# Makefile for vshell project

.PHONY: build test clean install generate-certs

# Variables
PROJECT = vshellProject
SERVER_BIN = vshell-server
CLIENT_BIN = vshell-client
GO_VERSION = 1.21

# Default target
all: build

# Build both binaries
build: build-server build-client

# Build server
build-server:
	@echo "Building server..."
	go build -o $(SERVER_BIN) ./cmd/vshell-server

# Build client
build-client:
	@echo "Building client..."
	go build -o $(CLIENT_BIN) ./cmd/vshell-client

# Run tests
test:
	@echo "Running tests..."
	go test -v ./...

# Run integration tests only
test-integration:
	@echo "Running integration tests..."
	go test -v ./tests/integration/...

# Run benchmarks
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. ./tests/benchmark/...

# Race detection test
test-race:
	@echo "Running race detector tests..."
	go test -race ./...

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(SERVER_BIN) $(CLIENT_BIN)
	go clean

# Install dependencies
deps:
	@echo "Tidying dependencies..."
	go mod tidy
	go mod download

# Generate TLS certificates for testing
generate-certs:
	@echo "Generating test certificates..."
	@mkdir -p certs
	@echo "Generating CA..."
	@openssl req -x509 -newkey rsa:4096 -keyout certs/ca.key -out certs/ca.crt -days 1 -nodes -subj "/CN=vshell-test-ca" 2>/dev/null
	@echo "Generating server certificate..."
	@openssl req -newkey rsa:4096 -keyout certs/server.key -out certs/server.csr -nodes -subj "/CN=vshell-server" 2>/dev/null
	@openssl x509 -req -in certs/server.csr -CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial -out certs/server.crt -days 1 2>/dev/null
	@echo "Generating client certificate..."
	@openssl req -newkey rsa:4096 -keyout certs/client.key -out certs/client.csr -nodes -subj "/CN=vshell-client" 2>/dev/null
	@openssl x509 -req -in certs/client.csr -CA certs/ca.crt -CAkey certs/ca.key -CAcreateserial -out certs/client.crt -days 1 2>/dev/null
	@echo "Certificates generated in certs/"

# Run server with test certificates
run-server: build generate-certs
	./$(SERVER_BIN) -a localhost:2222 \
		-c certs/server.crt \
		-k certs/server.key \
		--ca certs/ca.crt \
		--mtls \
		-l DEBUG

# Run client with test certificates
run-client: build generate-certs
	./$(CLIENT_BIN) -a localhost:2222 \
		-c certs/client.crt \
		-k client.key \
		--ca certs/ca.crt \
		-l DEBUG

# Code quality checks
fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

# Lint (requires golint)
lint:
	@which golint > /dev/null 2>&1 || (echo "golint not installed, run: go get -u golang.org/x/lint/golint" && exit 1)
	golint ./...

# Coverage report
coverage:
	@echo "Generating coverage report..."
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

# CI/CD targets
ci: deps fmt vet test test-race

# Development helpers
run-tests: test

rebuild: clean build
