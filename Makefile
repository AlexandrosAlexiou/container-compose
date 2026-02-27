BINARY_NAME=container-compose
BUILD_DIR=bin

.PHONY: build clean install test test-integration

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/container-compose

clean:
	rm -rf $(BUILD_DIR)

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

test:
	go test ./...

test-integration: build
	go test -tags integration -v -timeout 600s ./internal/integration/

test-integration-short: build
	go test -tags integration -short -v -timeout 300s ./internal/integration/

lint:
	golangci-lint run ./...
