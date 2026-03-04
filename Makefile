.PHONY: fmt test vet build ci image clean

BINARY=sssstore
PKG=./cmd/sssstore

fmt:
	gofmt -w $(shell find . -name "*.go" -type f)

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -trimpath -ldflags "-s -w" -o bin/$(BINARY) $(PKG)

ci: fmt vet test build

image:
	docker build -t sssstore:local .

clean:
	rm -rf bin
