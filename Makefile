BINARY := yaml-ls-k8s
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install test test-unit test-e2e clean

build:
	go build -o $(BINARY) ./cmd/yaml-ls-k8s

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

test: test-unit test-e2e

test-unit:
	go test ./internal/lsp/ ./internal/proxy/ ./internal/detector/... ./internal/schema/

test-e2e: build
	go test ./internal/e2e/ -timeout 120s

clean:
	rm -f $(BINARY)
