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

# Hand the export over as a single file. Copying the directory across stages
# exposes its contents to the .dockerignore patterns, and the app has a route
# called /images, which collides with the images/ entry that keeps multi-gigabyte
# ESXi ISOs out of the build context. The result was one silently missing page.
RUN tar -cf /ui.tar -C out .

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
COPY --from=ui /ui.tar /tmp/ui.tar
RUN rm -rf webui/dist && mkdir webui/dist && tar -xf /tmp/ui.tar -C webui/dist

# Every route under ui/src/app must have come through. Without this the build
# happily embeds an incomplete UI and the missing page only shows up as a 404
# in production, long after the build that caused it.
RUN for dir in ui/src/app/*/; do \
        route=$(basename "$dir"); \
        test -f "webui/dist/$route/index.html" \
            || { echo "embedded UI is missing the $route page"; exit 1; }; \
    done

# -buildvcs stamps the commit from the .git in the context, so /v1/version
# reports the build rather than "none". It is the default, but stated here
# because the whole point of shipping .git into the context is this flag.
RUN go build -buildvcs=auto -ldflags="-s -w" -o /out/go-via ./cmd/go-via

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
# 443/tcp  API and UI
#
# Decorative under network_mode: host, but a record of what is listening.
EXPOSE 67/udp 69/udp 80 443

ENTRYPOINT ["go-via"]
