# syntax=docker/dockerfile:1.12.0
ARG GO_VERSION=1.26.10
ARG REGISTRY="docker.io"

# Build-time only (never shipped), so pin exactly; bump tag and digest together.
FROM --platform=$BUILDPLATFORM ${REGISTRY}/tonistiigi/xx:1.10.0@sha256:40fe21f44bb8cddf1eb1226db190d0a984118f62ee02a91b77d99448d28264a6 AS xx
FROM --platform=$BUILDPLATFORM ${REGISTRY}/golang:${GO_VERSION} AS builder

ARG TARGETOS
ARG TARGETARCH
ARG BUILD_MODE=release

COPY --from=xx / /

WORKDIR /app/aerospike-backup-service
COPY . .

RUN xx-go --wrap

RUN --mount=type=secret,id=GOPROXY <<-EOF
    if [ -s /run/secrets/GOPROXY ]; then
        export GOPROXY=$(cat /run/secrets/GOPROXY)
    fi
    go mod download
EOF

RUN --mount=type=secret,id=GOPROXY <<-EOF
    if [ -s /run/secrets/GOPROXY ]; then
        export GOPROXY=$(cat /run/secrets/GOPROXY)
    fi
    OS="${TARGETOS}" ARCH="${TARGETARCH}" make build BUILD_MODE="${BUILD_MODE}"
    xx-verify /app/aerospike-backup-service/build/target/aerospike-backup-service_${TARGETOS}_${TARGETARCH}
EOF

# Pinned to the Alpine 3.24 release line: the `3.24` tag always resolves to the
# newest 3.24.x patch release, so rebuilds pick up fixes without jumping to 3.25.
FROM ${REGISTRY}/alpine:3.24
ARG TARGETOS
ARG TARGETARCH

# Pick up package fixes published since the base image was built.
RUN apk upgrade --no-cache

# addgroup/adduser are BusyBox applets, so no extra packages are needed.
# The log directory matches the documented file-writer path, so a file log works
# without mounting a volume (the service checks it is writable at startup).
RUN addgroup -g 65532 -S abgroup && \
    adduser -S -u 65532 -G abgroup -h /home/absuser absuser && \
    install -d -o absuser -g abgroup -m 0750 /var/log/aerospike-backup-service

COPY --chown=absuser:absgroup --chmod=0755 --from=builder \
    /app/aerospike-backup-service/build/target/aerospike-backup-service_${TARGETOS}_${TARGETARCH} \
    /usr/bin/aerospike-backup-service

COPY --chown=absuser:absgroup --from=builder \
    /app/aerospike-backup-service/build/package/config/aerospike-backup-service.yml \
    /etc/aerospike-backup-service/aerospike-backup-service.yml

USER absuser

EXPOSE 8080
ENTRYPOINT ["aerospike-backup-service"]
CMD ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml"]
