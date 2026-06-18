.PHONY: build run dev test lint clean frontend backend

BINARY=flux
GOFLAGS=-trimpath
LDFLAGS=-s -w

build: backend
	@if [ -f web/package.json ]; then $(MAKE) frontend; fi

backend:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/flux

frontend:
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
