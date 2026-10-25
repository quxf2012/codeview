VERSION ?= 0.4.0
LDFLAGS := -s -w -X main.version=$(VERSION)
export CGO_ENABLED=0

.PHONY: build run dist clean

build:
	go build -mod=vendor -ldflags "$(LDFLAGS)" -o codeview .

run: build
	./codeview -root $${ROOT:-.}

dist:
	mkdir -p dist
	GOOS=linux   GOARCH=amd64 go build -mod=vendor -ldflags "$(LDFLAGS)" -o dist/codeview-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -mod=vendor -ldflags "$(LDFLAGS)" -o dist/codeview-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -mod=vendor -ldflags "$(LDFLAGS)" -o dist/codeview-windows-amd64.exe .
	GOOS=darwin  GOARCH=arm64 go build -mod=vendor -ldflags "$(LDFLAGS)" -o dist/codeview-darwin-arm64 .

clean:
	rm -rf codeview dist
