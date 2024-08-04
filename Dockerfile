# syntax=docker/dockerfile:1
FROM registry.access.redhat.com/ubi9:latest as base
RUN dnf -y install make git && dnf clean all

ARG GO_VERSION=1.22.5
WORKDIR /app

RUN arch=${TARGETARCH:-amd64} \
    && curl -Lo /tmp/go.tgz "https://go.dev/dl/go${GO_VERSION}.linux-${arch}.tar.gz" \
    && tar -xzf /tmp/go.tgz -C /usr/local/ \
    && rm /tmp/go.tgz

FROM base as builder
ENV PATH="$PATH:/usr/local/go/bin"
ENV GOPATH=/app/${project}
ENV GOCACHE=/app/

WORKDIR /app/${project}
COPY . .

RUN --mount=type=secret,id=GITHUB_TOKEN <<-EOF
    go env -w GOPRIVATE="github.com/aerospike/backup-go"
    GITHUB_TOKEN="$(cat "/run/secrets/GITHUB_TOKEN")"
    git config --global url."https://${GITHUB_TOKEN}:x-oauth-basic@github.com/".insteadOf "https://github.com/"
    make build
EOF

FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /app/${project}/target/${project} /usr/bin/${project}
COPY --from=builder /app/${project}/config/config.yml /etc/${project}/${project}.yml

EXPOSE 8080
ENV DOCKER_CONTAINER=true

ENTRYPOINT ["aerospike-backup-service"]
CMD ["-c", "/etc/aerospike-backup-service/aerospike-backup-service.yml"]
