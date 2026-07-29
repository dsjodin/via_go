# syntax=docker/dockerfile:1

# A self-contained build: clone the repository and `docker compose up -d`.
# Nothing is pulled from a registry, because this project does not publish
# images.

# ---- frontend -------------------------------------------------------------
# Built first so that changes to the Go sources do not invalidate the npm
# install layer.
FROM node:22-bookworm-slim AS ui

WORKDIR /src/ui
COPY ui/package.json ui/package-lock.json ./
RUN npm ci

COPY ui/ ./
# next.config.mjs sets output: "export", so this produces a static ui/out.
RUN npm run build

# ---- backend --------------------------------------------------------------
FROM golang:1.25-bookworm AS build

WORKDIR /src

# go-sqlite3 is cgo, which is why the runtime stage below needs a libc rather
# than being scratch or distroless/static.
ENV CGO_ENABLED=1

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# webui/dist holds a committed placeholder page so that `go build` works
# without a Node toolchain. Replace it with the real export.
RUN rm -rf webui/dist
COPY --from=ui /src/ui/out ./webui/dist

RUN go build -ldflags="-s -w" -o /out/go-via ./cmd/go-via

# ---- runtime --------------------------------------------------------------
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# The binary resolves database/, images/, cert/ and secret/ relative to its
# working directory, so this is what the compose volumes bind to.
WORKDIR /app

COPY --from=build /out/go-via /usr/local/bin/go-via

# 67/udp   DHCP
# 69/udp   TFTP
# 80/tcp   UEFI HTTP boot
# 8443/tcp API and UI
#
# Decorative under network_mode: host, but a record of what is listening.
EXPOSE 67/udp 69/udp 80 8443

ENTRYPOINT ["go-via"]
