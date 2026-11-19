# LeeDevKit Go Container Image
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-bookworm

WORKDIR /workspace

RUN go version && gofmt -h >/dev/null && go help vet >/dev/null
