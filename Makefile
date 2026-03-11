.PHONY: build install clean test

BINARY   := jira-anvil
BIN_DIR  := bin
CMD_DIR  := cmd/jira-anvil
VERSION  := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS  := -ldflags "-X main.version=$(VERSION)"

# Build the binary into bin/
build:
	@mkdir -p $(BIN_DIR)
	cd $(CMD_DIR) && go build $(LDFLAGS) -o ../../$(BIN_DIR)/$(BINARY) .
	@echo "Built $(BIN_DIR)/$(BINARY)"

# Install the binary to GOPATH/bin (go install)
install:
	cd $(CMD_DIR) && go install $(LDFLAGS) .
	@echo "Installed $(BINARY) to $$(go env GOPATH)/bin/"

# Run tests
test:
	cd $(CMD_DIR) && go test ./...

# Remove built binary
clean:
	rm -f $(BIN_DIR)/$(BINARY)

# Update Go dependencies
deps:
	cd $(CMD_DIR) && go mod tidy

# Show help
help:
	@echo "Available targets:"
	@echo "  build    Build $(BINARY) to $(BIN_DIR)/"
	@echo "  install  Install $(BINARY) via go install"
	@echo "  test     Run tests"
	@echo "  clean    Remove built binary"
	@echo "  deps     Tidy Go dependencies"
