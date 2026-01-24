BINARY := settle

.PHONY: build install test clean

build:
	go build -o $(BINARY) .

install:
	go install .

test:
	go test -v ./...

clean:
	rm -f $(BINARY)
