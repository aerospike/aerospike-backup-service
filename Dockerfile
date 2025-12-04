# syntax=docker/dockerfile:1.12.0
ARG GO_VERSION=1.24.11
ARG REGISTRY="docker.io"

FROM --platform=$BUILDPLATFORM ${REGISTRY}/tonistiigi/xx AS xx
FROM --platform=$BUILDPLATFORM ${REGISTRY}/golang:${GO_VERSION} AS builder

ARG TARGETOS
ARG TARGETARCH

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
    OS=${TARGETOS} ARCH=${TARGETARCH} make build
    xx-verify /app/aerospike-backup-service/build/target/aerospike-backup-service_${TARGETOS}_${TARGETARCH}
EOF

FROM ${REGISTRY}/alpine:latest
ARG TARGETOS
ARG TARGETARCH

RUN apk update &&  \
    apk upgrade --no-cache

RUN apk add --no-cache shadow && \
    addgroup -g 65532 -S abgroup && \
    adduser -S -u 65532 -G abgroup -h /home/absuser absuser

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
