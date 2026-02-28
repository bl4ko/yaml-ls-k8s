BINARY := yaml-ls-k8s
INSTALL_DIR := $(HOME)/.local/bin

.PHONY: build install test clean

build:
	go build -o $(BINARY) ./cmd/yaml-ls-k8s

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY) $(INSTALL_DIR)/$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
