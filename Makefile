.PHONY: lint test build

lint:
	golangci-lint run ./...

test:
	go test -v -race -shuffle=on -timeout=1m -count=1 ./...

build:
	go build -o settle .
