.PHONY: wire mocks

wire:
	go tool wire ./cmd/dmc-dataloader-api

mocks:
	go tool mockery
