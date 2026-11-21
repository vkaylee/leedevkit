FROM golang:1.26-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /out/leedevkit-debug ./cmd/debug-mcp

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/leedevkit-debug /app/leedevkit-debug
USER nonroot:nonroot
ENTRYPOINT ["/app/leedevkit-debug"]
