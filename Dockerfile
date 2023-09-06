FROM --platform=${BUILDPLATFORM} golang:1.21.0-bookworm AS build

ARG TARGETOS
ARG TARGETARCH

ENV GOOS=${TARGETOS}
ENV GOARCH=${TARGETARCH}
ENV CGO_ENABLED=0
RUN mkdir -p /go/src/github.com/fujiwara/greenlight
COPY . /go/src/github.com/fujiwara/greenlight
WORKDIR /go/src/github.com/fujiwara/greenlight
RUN make

FROM debian:bookworm-slim
LABEL maintainer "fujiwara <fujiwara.shunichiro@gmail.com>"

RUN apt-get update && apt-get install -y \
    curl \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /go/src/github.com/fujiwara/greenlight /usr/local/bin/greenlight
RUN mkdir /opt/greenlight
# ADD greenlight.yaml /opt/greenlight/greenlight.yaml
WORKDIR /opt/greenlight

ENV GREENLIGHT_CONFIG=/opt/greenlight/greenlight.yaml
ENV GREENLIGHT_DEBUG=false
ENTRYPOINT ["/usr/local/bin/greenlight"]
