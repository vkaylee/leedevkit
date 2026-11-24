# LeeDevKit Go Container Image
ARG GO_VERSION=1.26
FROM golang:${GO_VERSION}-bookworm

RUN apt-get update && apt-get install -y --no-install-recommends \
    bash coreutils curl findutils grep gzip python3 sed tar \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

RUN go version && gofmt -h >/dev/null && go help vet >/dev/null && python3 --version && bash --version >/dev/null && curl --version >/dev/null && tar --version >/dev/null
