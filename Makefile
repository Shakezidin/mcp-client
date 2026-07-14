.DEFAULT_GOAL := help

GO := go
BINARY_DIR := bin
CMD_DIR := ./cmd

.PHONY: help tidy fmt test build-demo build-prod run run-prod clean

help:
	@echo "Usage:"
	@echo "  make build-demo      Build demo CLI binary"
	@echo "  make build-prod      Build production CLI binary"
	@echo "  make test            Run all Go tests"
	@echo "  make tidy            Run go mod tidy"
	@echo "  make fmt             Format Go source files"
	@echo "  make run             Build and run demo CLI"
	@echo "  make run-prod        Build and run production CLI"
	@echo "  make clean           Remove built binaries"

build-demo: $(BINARY_DIR)/mcpclient

build-prod: $(BINARY_DIR)/mcpclient_prod

$(BINARY_DIR)/mcpclient:
	mkdir -p $(BINARY_DIR)
	$(GO) build -o $@ $(CMD_DIR)/mcpclient

$(BINARY_DIR)/mcpclient_prod:
	mkdir -p $(BINARY_DIR)
	$(GO) build -o $@ $(CMD_DIR)/mcpclient/prod

test:
	$(GO) test ./...

tidy:
	$(GO) mod tidy

fmt:
	$(GO) fmt ./...

run: build-demo
	MCP_ENDPOINT=${MCP_ENDPOINT:-http://localhost:8080/ai-mb-mcp/v0beta/mcp} \
	OTEL_ENDPOINT=${OTEL_ENDPOINT:-localhost} \
	OTEL_PORT=${OTEL_PORT:-4317} \
	./$(BINARY_DIR)/mcpclient

run-prod: build-prod
	MCP_ENDPOINT=${MCP_ENDPOINT:-http://localhost:8080/ai-mb-mcp/v0beta/mcp} \
	OTEL_ENDPOINT=${OTEL_ENDPOINT:-localhost} \
	OTEL_PORT=${OTEL_PORT:-4317} \
	./$(BINARY_DIR)/mcpclient_prod

clean:
	rm -rf $(BINARY_DIR)
