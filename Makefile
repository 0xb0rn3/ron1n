VERSION ?= 0.0.1zoro

.PHONY: all build test race vet release clean

all: test build

build:
	go build -trimpath -o build/ron1n ./cmd/ron1n
	go build -trimpath -o build/ron1n-relay ./cmd/ron1n-relay

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

release:
	RON1N_VERSION=$(VERSION) ./scripts/build-release.sh dist/$(VERSION)

clean:
	go clean
