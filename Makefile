# EVOLV — build the organism
.PHONY: build install run release cross clean test vet fmt check

VERSION ?= $(shell git describe --tags --always 2>/dev/null || echo "0.1.0")

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X github.com/dxn111/EVOLV/internal/workspace.Version=$(VERSION)" -o evolv ./cmd/evolv

run: build
	./evolv

install: build
	install -m 0755 evolv $(HOME)/.local/bin/evolv

# the same gate CI runs: format, vet, tests
test:
	go test ./... -count=1

vet:
	go vet ./...

fmt:
	gofmt -w $$(git ls-files '*.go')

check:
	@test -z "$$(gofmt -l .)" || { echo "gofmt needed:"; gofmt -l .; exit 1; }
	go vet ./...
	go test ./... -count=1

# cross-compile all release targets (CGO-free: pure Go + goja)
cross:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/evolv_darwin_amd64  ./cmd/evolv
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/evolv_darwin_arm64  ./cmd/evolv
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/evolv_linux_amd64   ./cmd/evolv
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o dist/evolv_linux_arm64   ./cmd/evolv
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm   go build -trimpath -ldflags "-s -w" -o dist/evolv_linux_arm     ./cmd/evolv
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o dist/evolv_windows_amd64.exe ./cmd/evolv

release: cross
	cd dist && for f in evolv_*; do tar -czf $$f.tar.gz $$f; done && shasum -a 256 *.tar.gz > checksums.txt

clean:
	rm -rf dist evolv
