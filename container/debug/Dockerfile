FROM python:3.12-slim

RUN useradd --uid 65532 --create-home --shell /usr/sbin/nologin debug
WORKDIR /app
COPY scripts/_debug_manifest.py scripts/_debug_mcp.py scripts/_debug_runtime.py /app/
USER 65532:65532
ENTRYPOINT ["python", "/app/_debug_mcp.py"]
