BINDIR := bin

.PHONY: proto build server agent dev tidy

proto:
	@echo "[proto] generating code via buf..."
	buf generate

server:
	@echo "[build] server"
	GOFLAGS=-mod=mod go build -o $(BINDIR)/croupier-server ./cmd/server

agent:
	@echo "[build] agent"
	GOFLAGS=-mod=mod go build -o $(BINDIR)/croupier-agent ./cmd/agent

build: server agent

tidy:
	go mod tidy

dev: build
	@echo "Run binaries in two shells or via supervisor with your TLS config."

