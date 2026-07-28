# goreleaser supplies the already-built binary, so this only needs a runtime
# base. go-sqlite3 is cgo, so the image needs a libc.
FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY go-via /usr/local/bin/go-via

# 67/udp   DHCP
# 69/udp   TFTP
# 80/tcp   UEFI HTTP boot
# 8443/tcp API and UI
EXPOSE 67/udp 69/udp 80 8443

ENTRYPOINT ["go-via"]
