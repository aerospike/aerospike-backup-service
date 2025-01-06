# syntax=docker/dockerfile:1.12.0
ARG GO_VERSION=1.23.4
ARG REGISTRY="docker.io"
FROM --platform=$BUILDPLATFORM ${REGISTRY}/tonistiigi/xx AS xx
FROM --platform=$BUILDPLATFORM ${REGISTRY}/golang:${GO_VERSION} AS builder

ARG TARGETOS
ARG TARGETARCH

COPY --from=xx / /

WORKDIR /app/aerospike-backup-service
COPY . .

RUN <<-EOF
    xx-go --wrap
    OS=${TARGETOS} ARCH=${TARGETARCH} make build
    xx-verify /app/aerospike-backup-service/build/target/aerospike-backup-service_${TARGETOS}_${TARGETARCH}
EOF

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
ARG TARGETOS
ARG TARGETARCH
COPY --from=builder /app/aerospike-backup-service/build/target/aerospike-backup-service_${TARGETOS}_${TARGETARCH} /usr/bin/aerospike-backup-service
COPY --from=builder /app/aerospike-backup-service/config/config.yml /etc/aerospike-backup-service/aerospike-backup-service.yml

EXPOSE 8080

ENTRYPOINT ["aerospike-backup-service"]
CMD ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml"]
