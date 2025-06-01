.PHONY: test build clean all

BINARY := aws-init
LDFLAGS := -ldflags="-s -w"

build:
	go build $(LDFLAGS) -o $(BINARY)

test:
	go test -v

clean:
	rm -f $(BINARY)

all: clean test build

linux:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY)-linux-amd64

darwin:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o $(BINARY)-darwin-arm64

release: clean test linux darwin
