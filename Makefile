.POSIX:
.SUFFIXES:

.PHONY: default
default: build

.PHONY: build
build:
	go build -o pg2s3 main.go

.PHONY: test
test:
	go test -count=1 ./...

.PHONY: cover
cover:
	go test -coverprofile=c.out -coverpkg=./... -count=1 ./...
	go tool cover -html=c.out

.PHONY: release
release:
	goreleaser release --snapshot --rm-dist

.PHONY: lint
lint:
	go run github.com/golangci/golangci-lint/cmd/golangci-lint@latest run --fast

.PHONY: format
format:
	gofmt -l -s -w .

.PHONY: update
update:
	go get -u ./...
	go mod tidy

.PHONY: clean
clean:
	rm -fr pg2s3 c.out dist/
