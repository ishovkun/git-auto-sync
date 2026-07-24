.PHONY: lint test install

GO_BIN := $(shell go env GOBIN)
ifeq ($(strip $(GO_BIN)),)
GO_BIN := $(shell go env GOPATH)/bin
endif
	
lint:
	golangci-lint run

test:
	go test ./...

install:
	go install .
	mkdir -p "$(GO_BIN)"
	go build -o "$(GO_BIN)/git-auto-sync-daemon" ./daemon
