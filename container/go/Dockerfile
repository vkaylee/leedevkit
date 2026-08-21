# LeeDevKit Go Container Image
ARG GO_VERSION=1.24
FROM golang:${GO_VERSION}-bookworm

WORKDIR /workspace

RUN go version && gofmt -h >/dev/null && go vet -h >/dev/null
