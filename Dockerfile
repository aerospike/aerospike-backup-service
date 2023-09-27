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
RUN make EVENT_LIB=libuv
RUN ../../scripts/copy_shared.sh

ENV GOOS linux

WORKDIR /app/cmd/backup
RUN go mod download
RUN CGO_ENABLED=1 go build .

FROM scratch

COPY --from=build /app/cmd/backup/backup .
COPY --from=build /app/cmd/backup/config/config.yml .

EXPOSE 8080
ENTRYPOINT ["./backup", "-c", "config.yml"]
