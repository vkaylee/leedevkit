FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY leedevkit-debug /app/leedevkit-debug
USER nonroot:nonroot
ENTRYPOINT ["/app/leedevkit-debug"]
