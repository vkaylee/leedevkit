# =============================================================================
# LeeDevKit Rust Container Image
# =============================================================================
# Default Dockerfile for Rust projects. Provides a complete Rust toolchain
# with all tools needed by the devkit test pipeline (fmt, clippy, test, coverage).
#
# Rust version is configurable via the RUST_VERSION build arg (default: 1.85).
# Override in leedevkit.toml:
#   [services.rust]
#   rust_version = "1.83"
# Or via environment variable:
#   RUST_VERSION=1.83 ./leedevkit test all
#
# Used by container/docker-compose.test.yml for single-container Rust projects.
# For multi-service projects, create your own Dockerfile at the project root.
# =============================================================================

ARG RUST_VERSION=1.85
FROM rust:${RUST_VERSION}-slim-bookworm

# ── System dependencies for common Rust crates ──
RUN apt-get update && apt-get install -y --no-install-recommends \
    pkg-config \
    libssl-dev \
    curl \
    && rm -rf /var/lib/apt/lists/*

# ── Rust components ──
RUN rustup component add \
    clippy \
    rustfmt \
    llvm-tools-preview

# ── Cargo tools (prebuilt binaries for fast builds) ──
# cargo-nextest: faster test runner
RUN curl -LsSf https://get.nexte.st/latest/linux | tar zxf - -C /usr/local/cargo/bin

# cargo-llvm-cov: coverage via LLVM source-based profiling
RUN cargo install cargo-llvm-cov

# ── Workspace ──
WORKDIR /workspace

# Verify toolchain
RUN cargo --version \
    && cargo fmt --version \
    && cargo clippy --version \
    && cargo nextest --version \
    && cargo llvm-cov --version
