BINARY := jeed
VERSION := 0.1.0
LDFLAGS := -s -w -X github.com/cosmos/cosmos-sdk/version.Name=jeechain -X github.com/cosmos/cosmos-sdk/version.AppName=jeed -X github.com/cosmos/cosmos-sdk/version.Version=$(VERSION)
BUILD_FLAGS := -trimpath -ldflags '$(LDFLAGS)'
LINUX_ARM64 := build/jeed-linux-arm64

.PHONY: all build build-linux-arm64 build-linux-arm64-upx install test fmt vet lint tidy proto init start clean help

all: tidy fmt vet test build ## Run the full local pipeline

build: ## Compile the jeed node binary into ./build (local OS/arch)
	go build $(BUILD_FLAGS) -o build/$(BINARY) ./cmd/jeed

build-linux-arm64: ## Cross-compile stripped ARM64 Linux binary (~70 MB)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(BUILD_FLAGS) -o $(LINUX_ARM64) ./cmd/jeed

build-linux-arm64-upx: build-linux-arm64 ## UPX-compress ARM64 binary (~16 MB; test on Oracle first)
	@command -v upx >/dev/null 2>&1 || { echo "upx not found — install from https://upx.github.io"; exit 1; }
	upx --best $(LINUX_ARM64)

install: ## Install jeed into $GOPATH/bin
	go install $(BUILD_FLAGS) ./cmd/jeed

test: ## Run the test suite
	go test ./... -count=1

fmt: ## Format all Go sources
	gofmt -s -w .

vet: ## Run go vet static checks
	go vet ./...

lint: ## Run golangci-lint (must be installed)
	golangci-lint run ./...

tidy: ## Sync go.mod / go.sum
	go mod tidy

proto: ## Generate code from protobuf definitions
	buf generate

init: build ## Initialize a local single-node devnet
	./build/$(BINARY) init localnode --chain-id JEE --home ./.jeechain
	cp config/genesis.json ./.jeechain/config/genesis.json
	cp config/config.toml ./.jeechain/config/config.toml
	cp config/app.toml ./.jeechain/config/app.toml
	@echo "Devnet initialized at ./.jeechain — run 'make gentx' to add a validator"

gentx: build ## Create a validator gentx (set VALIDATOR_NAME and DELEGATION)
	./build/$(BINARY) keys add $(VALIDATOR_NAME) --keyring-backend test --home ./.jeechain
	./build/$(BINARY) genesis gentx $(VALIDATOR_NAME) $(DELEGATION) \
		--chain-id JEE --keyring-backend test --home ./.jeechain
	./build/$(BINARY) genesis collect-gentxs --home ./.jeechain
	./build/$(BINARY) genesis validate --home ./.jeechain

start: ## Start the local node
	./build/$(BINARY) start --home ./.jeechain

clean: ## Remove build artifacts and local devnet data
	rm -rf build/ .jeechain/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'
