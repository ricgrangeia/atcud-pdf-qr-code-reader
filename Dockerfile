### STAGE 1 — Build ###

FROM golang:1.25.9-alpine AS build

# CGO is disabled for a fully static binary.
ENV GOOS=linux GOARCH=amd64 CGO_ENABLED=0

WORKDIR /app

# Copy dependency manifests first so Docker can cache the download layer.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source.
# No code-generation step needed — Huma derives the OpenAPI spec at runtime.
COPY . .

# Compile the binary.
RUN go build -ldflags="-w -s" -o /app/server ./cmd/go_api


### STAGE 2 — Runtime ###

FROM ubuntu:jammy AS runtime

# poppler-utils provides pdftoppm, used to render PDF pages to PNG images.
# ca-certificates is needed for HTTPS calls (e.g. to the vLLM service).
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        ca-certificates \
        poppler-utils \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

# Create a non-root user for security.
ENV USER=gouser UID=10001
RUN adduser \
    --disabled-password \
    --gecos "" \
    --shell "/bin/bash" \
    --uid "${UID}" \
    "${USER}"

# Copy the compiled binary from the build stage.
COPY --from=build --chown=${USER}:${USER} /app/server ./bin/server

USER ${USER}:${USER}

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
    CMD pgrep -f server > /dev/null || exit 1

CMD ["./bin/server"]
