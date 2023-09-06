LATEST_TAG := $(shell git describe --tags)
TAG ?= latest

.PHONY: clean test

greenlight: go.* *.go
	go build \
		-ldflags "-s -w -X main.Version=$(LATEST_TAG)" \
		-o $@ \
		cmd/greenlight/main.go

clean:
	rm -rf greenlight dist/ vendor/

test:
	go test -v ./...

install:
	go install \
		-ldflags "-s -w -X main.Version=$(LATEST_TAG)" \
		./cmd/greenlight

dist:
	goreleaser build --snapshot --rm-dist

docker-build-and-push:
	go mod vendor
	docker buildx build \
		--build-arg version=${LATEST_TAG} \
		--platform=linux/amd64,linux/arm64 \
		-t ghcr.io/fujiwara/greenlight:${LATEST_TAG} \
		-f Dockerfile \
		--push \
		.
