.PHONY: check test fmt

check:
	cd src/server && test -z "$$(gofmt -l .)" && go vet ./... && go test ./...

test:
	cd src/server && go test ./...

fmt:
	gofmt -w src/server
