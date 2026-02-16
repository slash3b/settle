BINARY := settle
BUILD_TIME := $(shell date -u '+%Y-%m-%d %H:%M:%S UTC')
LDFLAGS := -X 'main.BuildTime=$(BUILD_TIME)'

.PHONY: build
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

.PHONY: install
install:
	go install -ldflags="$(LDFLAGS)" .

.PHONY: test
test:
	go test -v -race -shuffle=on -timeout=1m -count=1

.PHONY: cover
cover:
	go test -race -failfast -shuffle=on -timeout=1m -count=1 -cover -coverprofile=out.html
	go tool cover -html=out.html

.PHONY: fmt
fmt:
	gofumpt -w .
	gci write \
		--custom-order \
		--section standard \
		--section default \
		--section blank \
		--section dot \
		--skip-generated \
		.

.PHONY: lint
lint:
	@golangci-lint run
