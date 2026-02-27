BINARY_NAME=container-compose
BUILD_DIR=bin

.PHONY: build clean install test

build:
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/container-compose

clean:
	rm -rf $(BUILD_DIR)

install: build
	cp $(BUILD_DIR)/$(BINARY_NAME) /usr/local/bin/

test:
	go test ./...

lint:
	golangci-lint run ./...
