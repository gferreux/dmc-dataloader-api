.PHONY: wire mocks test lint

wire:
	go tool wire ./cmd/dmc-dataloader-api

mocks:
	go tool mockery

test:
	go test ./...

lint:
	golangci-lint run
