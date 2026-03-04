.PHONY: check-conflicts fmt test vet build ci qa image clean

BINARY=sssstore
PKG=./cmd/sssstore

check-conflicts:
	@if rg -n "^(<<<<<<<|=======|>>>>>>>)" --glob '!*.md' --glob '!*go.sum' .; then \
		echo "merge conflict markers found"; \
		exit 1; \
	fi

fmt:
	gofmt -w $(shell find . -name "*.go" -type f)

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -trimpath -ldflags "-s -w" -o bin/$(BINARY) $(PKG)

ci: check-conflicts fmt vet test build

qa:
	./scripts/qa.sh

image:
	docker build -t sssstore:local .

clean:
	rm -rf bin
