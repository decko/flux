.PHONY: build run dev test lint clean frontend backend all migrate seed

BINARY=flux
GOFLAGS=-trimpath
LDFLAGS=-s -w

# default target — full production build (frontend + backend)
all: build

# frontend build — compiles TypeScript/React into web/dist/
frontend:
	cd web && bun install && bun run build

# backend build — Go binary WITHOUT embedded frontend (fast iteration)
backend:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/flux

# full build — frontend FIRST, then backend with embedded SPA
build: frontend
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/flux

run:
	./bin/$(BINARY)

# dev — rebuilds frontend first, then hot-runs the backend
dev: frontend
	go run ./cmd/flux

test:
	go test -race -cover ./...

lint:
	golangci-lint run

clean:
	rm -rf bin/ web/dist/ web/node_modules/

migrate:
	go run ./cmd/flux migrate

seed:
	go run ./cmd/flux seed

set-password:
	go run ./cmd/flux user set-password

user-add:
	go run ./cmd/flux user add
