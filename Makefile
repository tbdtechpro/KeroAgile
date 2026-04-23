.PHONY: build install test clean

build:
	go build -o KeroAgile ./cmd/keroagile/

install:
	go build -o $(HOME)/.local/bin/KeroAgile ./cmd/keroagile/

test:
	go test ./...

clean:
	rm -f KeroAgile
