LATEST_TAG := $(shell git describe --tags)
TAG ?= latest
SHELL = /bin/bash

.PHONY: clean test

greenlight: go.* *.go
	go build \
		-ldflags "-s -w -X main.Version=$(LATEST_TAG)" \
		-o $@ \
		cmd/greenlight/main.go

clean:
	rm -rf greenlight dist/ vendor/ Dockerfile

test:
	go test -v ./...

install:
	go install \
		-ldflags "-s -w -X main.Version=$(LATEST_TAG)" \
		./cmd/greenlight

dist:
	goreleaser build --snapshot --rm-dist

Dockerfile:
	cat Dockerfile.build > Dockerfile
	echo "FROM $(BASE_IMAGE)" >> Dockerfile
	cat Dockerfile.target >> Dockerfile

docker-build-and-push: Dockerfile
	go mod vendor
	docker buildx build \
		--build-arg version=${LATEST_TAG} \
		--platform=linux/amd64,linux/arm64 \
		-t ghcr.io/fujiwara/greenlight:${LATEST_TAG}${IMAGE_SUFFIX} \
		-f Dockerfile \
		--push \
		.
