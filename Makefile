.PHONY: build run dev test lint clean frontend backend migrate seed

BINARY=flux
GOFLAGS=-trimpath
LDFLAGS=-s -w

build: web/dist
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/flux

backend:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/flux

web/dist:
	cd web && npm install && npm run build

run:
	./bin/$(BINARY)

dev:
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
