.PHONY: build build-web install test clean

build:
	go build -o KeroAgile ./cmd/keroagile/

# build-web: compile React app and copy into internal/web/dist for go:embed
build-web:
	cd web && npm run build
	rm -rf internal/web/dist
	cp -r web/dist internal/web/dist

# build-full: build-web first (embeds UI), then compile Go binary
build-full: build-web
	go build -o KeroAgile ./cmd/keroagile/

install:
	go build -o $(HOME)/.local/bin/KeroAgile ./cmd/keroagile/

test:
	go test $(shell go list ./... | grep -v '/web/')

clean:
	rm -f KeroAgile
	rm -rf internal/web/dist web/dist
