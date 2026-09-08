# syntax=docker/dockerfile:1
# Build from sdks/community/go so example/server's replace ../../ resolves:
#   docker build -f example/server/Dockerfile .
FROM golang:1.25.1-bookworm@sha256:c423747fbd96fd8f0b1102d947f51f9b266060217478e5f9bf86f145969562ee AS build

WORKDIR /src
COPY go.mod go.sum ./
COPY pkg/ ./pkg/
COPY example/server/go.mod example/server/go.sum ./example/server/
WORKDIR /src/example/server
RUN GOWORK=off go mod download
COPY example/server/ ./
RUN GOWORK=off go build -trimpath -o /out/server ./cmd

FROM debian:bookworm-slim@sha256:88200866dfff7ea7f5cbcb6ec7c8a701889efe6fe859fe64d6990e4b07ea4171
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=build --chown=65532:65532 /out/server /app/server
USER 65532:65532
ENV HOST=0.0.0.0 PORT=8080 AGENT_WORKSPACE=/app
EXPOSE 8080
ENTRYPOINT ["/app/server"]
