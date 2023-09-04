.PHONY: clean test

greenlight: go.* *.go
	go build -o $@ cmd/greenlight/main.go

clean:
	rm -rf greenlight dist/

test:
	go test -v ./...

install:
	go install github.com/fujiwara/greenlight/cmd/greenlight

dist:
	goreleaser build --snapshot --rm-dist
