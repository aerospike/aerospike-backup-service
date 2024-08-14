# syntax=docker/dockerfile:1.7-labs
FROM registry.access.redhat.com/ubi9:latest as base
RUN dnf -y install make git && dnf clean all

ARG GO_VERSION=1.22.6
WORKDIR /app

RUN arch=${TARGETARCH:-amd64} \
    && curl -Lo /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" \
    && tar -xzf /tmp/go.tgz -C /usr/local/ \
    && rm /tmp/go.tgz

FROM base as builder
ENV PATH="$PATH:/usr/local/go/bin"
ENV GOPATH=/app/aerospike-backup-service
ENV GOCACHE=/app/

WORKDIR /app/aerospike-backup-service
COPY . .

RUN make build

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /app/aerospike-backup-service/build/target/aerospike-backup-service /usr/bin/aerospike-backup-service
COPY --from=builder /app/aerospike-backup-service/config/config.yml /etc/aerospike-backup-service/aerospike-backup-service.yml

EXPOSE 8080

ENTRYPOINT ["aerospike-backup-service"]
CMD ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml"]
