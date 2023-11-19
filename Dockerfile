# syntax=docker/dockerfile:1
FROM golang:1.21 as build

RUN --mount=type=cache,target=/var/cache/apt,sharing=locked \
    --mount=type=cache,target=/var/lib/apt,sharing=locked \
    apt-get update \
    && apt-get install -y git build-essential libssl-dev libuv1-dev libcurl4-openssl-dev libzstd-dev \
    && apt-get install -y cmake pkg-config zlib1g-dev

WORKDIR /app
COPY . .

WORKDIR /app/modules/aerospike-tools-backup
RUN git submodule update --init --recursive
RUN git clone https://github.com/aws/aws-sdk-cpp.git

WORKDIR /app/modules/aerospike-tools-backup/aws-sdk-cpp
RUN git submodule update --init --recursive

RUN mkdir build
RUN cmake -S . -B build -DCMAKE_BUILD_TYPE=Release -DBUILD_ONLY="s3" -DBUILD_SHARED_LIBS=ON \
    -DENABLE_TESTING=OFF -DCMAKE_INSTALL_PREFIX=/usr/local -DCMAKE_INSTALL_LIBDIR=lib
RUN make -C build

WORKDIR /app/modules/aerospike-tools-backup/aws-sdk-cpp/build
RUN make install

WORKDIR /app/modules/aerospike-tools-backup
RUN make shared EVENT_LIB=libuv
RUN ../../scripts/copy_shared.sh

ARG GOOS linux
ARG GOARCH=$BUILDARCH

WORKDIR /app/cmd/backup
RUN go mod download

RUN export ARCH=`uname -m` && \
    CGO_CFLAGS="-I/app/modules/aerospike-tools-backup/modules/c-client/target/Linux-$ARCH/include \
    -I/app/modules/aerospike-tools-backup/include" \
    GOOS=$GOOS GOARCH=$GOARCH CGO_ENABLED=1 go build  \
    -ldflags " -X main.commit=$(git rev-parse --short HEAD) -X main.buildTime=$(date -u +'%Y-%m-%dT%H:%M:%SZ')" \
    -o backup .

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
COPY --from=build /app/config/config.yml /app/config.yml

EXPOSE 8080
ENTRYPOINT ["./backup", "-c", "config.yml"]
