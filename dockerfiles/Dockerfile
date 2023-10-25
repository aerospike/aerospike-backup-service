# syntax=docker/dockerfile:1
ARG BASE_IMAGE=abs-base-image
FROM $BASE_IMAGE as build

WORKDIR /app

COPY . .

ARG GOOS linux
ARG GOARCH=$BUILDARCH

WORKDIR /app/cmd/backup
RUN go mod download

RUN export ARCH=`uname -m` && \
    CGO_CFLAGS="-I/app/modules/aerospike-tools-backup/modules/c-client/target/Linux-$ARCH/include \
    -I/app/modules/aerospike-tools-backup/include" \
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=1 go build -o backup .

FROM ubuntu:22.04

RUN apt-get update \
    && apt-get install -y libssl-dev libuv1-dev libcurl4-openssl-dev libzstd-dev

WORKDIR /app
ENV LD_LIBRARY_PATH /app

COPY --from=build /usr/local/lib/libaws-cpp-sdk-core.so /app/libaws-cpp-sdk-core.so
COPY --from=build /usr/local/lib/libaws-cpp-sdk-s3.so /app/libaws-cpp-sdk-s3.so
COPY --from=build /app/lib/libasbackup.so /app/libasbackup.so
COPY --from=build /app/lib/libasrestore.so /app/libasrestore.so

COPY --from=build /app/cmd/backup/backup /app/backup
COPY --from=build /app/cmd/backup/config/config.yml /app/config.yml

EXPOSE 8080
ENTRYPOINT ["./backup", "-c", "config.yml"]
