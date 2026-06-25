# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM gcc:13 AS builder

WORKDIR /build

# Install sqlite3 dev headers for compilation
RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-dev \
    && rm -rf /var/lib/apt/lists/*

# Copy source
COPY src/ ./src/
COPY main.cpp .
COPY Makefile .

# Build the binary
RUN make

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM debian:bookworm-slim

WORKDIR /app

# Runtime deps:
#   libsqlite3  — SQLite fallback
#   python3     — code runner (python language support)
#   bash        — code runner (bash language support)
RUN apt-get update && apt-get install -y --no-install-recommends \
    libsqlite3-0 \
    python3 \
    bash \
    && rm -rf /var/lib/apt/lists/*

# Copy binary from builder
COPY --from=builder /build/server .

# SQLite DB persisted via volume
VOLUME ["/app/data"]

EXPOSE 9090

CMD ["./server"]
