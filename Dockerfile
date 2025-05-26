# syntax=docker/dockerfile:1.12.0
ARG GO_VERSION=1.23.9
ARG REGISTRY="docker.io"
ARG RH_REGISTRY="registry.access.redhat.com"

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

FROM ${RH_REGISTRY}/ubi9/ubi-minimal:latest
ARG TARGETOS
ARG TARGETARCH

RUN microdnf install -y shadow-utils && \
    microdnf update -y && \
    microdnf -y clean all && rm -rf /var/cache/yum && \
    groupadd --system --gid 65532 nonroot && \
    useradd --no-log-init --no-user-group --system --uid 65532 --gid 65532 nonroot

COPY --chown=nonroot:65532 --chmod=0755 --from=builder \
    /app/aerospike-backup-service/build/target/aerospike-backup-service_${TARGETOS}_${TARGETARCH} \
    /usr/bin/aerospike-backup-service

COPY --chown=nonroot:65532 --from=builder \
    /app/aerospike-backup-service/build/package/config/aerospike-backup-service.yml \
    /etc/aerospike-backup-service/aerospike-backup-service.yml

USER nonroot
EXPOSE 8080

ENTRYPOINT ["aerospike-backup-service"]
CMD ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml"]
