.PHONY: run build dev

export CGO_ENABLED := 0

ifneq (,$(wildcard .env))
include .env
export
endif

run:
	cd apps/backend && go run ./cmd/server

build:
	cd apps/backend && go build -o bin/server ./cmd/server

dev:
	cd apps/backend && air
