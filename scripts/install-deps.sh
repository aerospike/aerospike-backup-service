#!/bin/bash -e
WORKSPACE="$(pwd)"
#WORKSPACE="$(git rev-parse --show-toplevel)"

mkdir -p "$WORKSPACE/static"
STATIC_DIR="$WORKSPACE/static"

# Versions
AWS_CPP_SDK_VERSION="1.10.55"
LIBCURL_VERSION="curl-7_83_1"
LIBUV_VERSION="v1.42.0"
LIBJANSSON_VERSION="v2.14"
GOLANG_VERSION="1.21.5"

LIBZSTD_VERSION="v1.5.5"
OPENSSL_VERSION="openssl-3.0.8"


if [ "$(uname -s)" == "Darwin" ]; then
  brew install go libtool make cmake automake git openssl@1.1 libuv curl zstd libssh2 aws-sdk-cpp jansson
else
  DISTRO="$(cat /etc/os-release | grep ^ID_LIKE= /etc/os-release | cut -d'=' -f2)"

  if [ "$DISTRO" == "debian" ]; then
    apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y \
    make \
    cmake \
    autotools-dev \
    automake \
    libtool \
    build-essential \
    zlib1g-dev \
    pkg-config \
    debhelper \
    libssl-dev \
    libzstd-dev
  else
    dnf update && dnf install -y \
    make \
    cmake \
    automake \
    libtool \
    zlib-devel \
    git \
    g++ \
    rpmdevtools \
    rpmlint \
    perl \
    openssl-devel \
    libzstd-devel
#    sudo dnf update && sudo dnf install -y autoconf automake libtool cmake git perl g++
  fi

  # Install golang
  if ! command -v go >/dev/null 2>&1 || [ ! -d /usr/local/go/bin ]; then
      echo "Install go"
      OS=$(uname | tr '[:upper:]' '[:lower:]')
      ARCH=$(uname -m)

      if [ "$ARCH" = "x86_64" ]; then
          ARCH="amd64"
      elif [ "$ARCH" = "aarch64" ]; then
          ARCH="arm64"
      fi

      curl -LO "https://go.dev/dl/go$GOLANG_VERSION.$OS-$ARCH.tar.gz" > "go$GOLANG_VERSION.$OS-$ARCH.tar.gz"
      tar -C /usr/local -xzf "go$GOLANG_VERSION.$OS-$ARCH.tar.gz"
      rm "go$GOLANG_VERSION.$OS-$ARCH.tar.gz"
      if [ ! -f /.dockerenv ]; then
        echo "export PATH=$PATH:/usr/local/go/bin" >> "$HOME/.bashrc"
        source "$HOME/.bashrc"
      fi
  fi

  if [ -f "$WORKSPACE/ltmain.sh" ]; then
      rm "$WORKSPACE/ltmain.sh"
  fi

  # Install openssl
  if [ ! -d "$STATIC_DIR/openssl" ]; then
    cd "$STATIC_DIR" &&  git clone https://github.com/openssl/openssl.git
  fi
  cd "$STATIC_DIR/openssl" && \
  git fetch --all --tags && \
  git checkout tags/"$OPENSSL_VERSION" && \
  git submodule update --init --recursive && \
  ./config -no-pinshared -no-pinshared -no-tests && \
  make && \
  make install

  if [ -f "$WORKSPACE/ltmain.sh" ]; then
      rm "$WORKSPACE/ltmain.sh"
  fi

  # Install curl
  if [ ! -d "$STATIC_DIR/curl" ]; then
    cd "$STATIC_DIR" &&  git clone --recurse-submodules https://github.com/curl/curl.git
  fi
  cd "$STATIC_DIR/curl" && \
  git fetch --all --tags && \
  git checkout tags/"$LIBCURL_VERSION" && \
  autoreconf -i && \
  ./configure --disable-shared --enable-static --with-pic --with-openssl && \
  make && \
  make install

  if [ -f "$WORKSPACE/ltmain.sh" ]; then
      rm "$WORKSPACE/ltmain.sh"
  fi

  # Install zstd
  if [ ! -d "$STATIC_DIR/zstd" ]; then
    cd "$STATIC_DIR" &&  git clone --recurse-submodules https://github.com/facebook/zstd
  fi
  cd "$STATIC_DIR/zstd" && \
  git fetch --all --tags && \
  git checkout tags/"$LIBZSTD_VERSION" && \
  mkdir -p ./build/cmake/builddir && \
  cmake -S ./build/cmake -B ./build/cmake/builddir -DZSTD_BUILD_SHARED=OFF && \
  make -C build/cmake/builddir && \
  make -C build/cmake/builddir install

  if [ -f "$WORKSPACE/ltmain.sh" ]; then
      rm "$WORKSPACE/ltmain.sh"
  fi

  # Install aws-cpp-sdk
  if [ ! -d "$STATIC_DIR/aws-sdk-cpp" ]; then
    cd "$STATIC_DIR" && git clone --recurse-submodules https://github.com/aws/aws-sdk-cpp
  fi
  cd "$STATIC_DIR/aws-sdk-cpp" && \
  git fetch --all --tags && \
  git checkout tags/"$AWS_CPP_SDK_VERSION" && \
  cmake -S . -B build \
  -DCMAKE_BUILD_TYPE=Release \
  -DBUILD_ONLY="s3" \
  -DBUILD_SHARED_LIBS=OFF \
  -DENABLE_TESTING=OFF \
  -DCMAKE_INSTALL_PREFIX=/usr/local \
  -DCMAKE_INSTALL_LIBDIR=lib \
  -DENABLE_UNITY_BUILD=ON && \
  make -C build && \
  make -C build install


  # Install libuv
  if [ ! -d "$STATIC_DIR/libuv" ]; then
    cd "$STATIC_DIR" && git clone https://github.com/libuv/libuv.git
  fi
  cd "$STATIC_DIR/libuv" && \
  git fetch --all --tags && \
  git checkout tags/"$LIBUV_VERSION" && \
  sh autogen.sh && \
  ./configure --with-pic --disable-shared --enable-static && \
  make && \
  make install

  if [ -f "$WORKSPACE/ltmain.sh" ]; then
      rm "$WORKSPACE/ltmain.sh"
  fi

  # Install jansson
  if [ ! -d "$STATIC_DIR/jansson" ]; then
    cd "$STATIC_DIR" && git clone https://github.com/akheron/jansson.git
  fi
  cd "$STATIC_DIR/jansson" && \
  git fetch --all --tags && \
  git checkout tags/"$LIBJANSSON_VERSION" && \
  autoreconf -i && \
  ./configure --with-pic --enable-shared=no --enable-static=yes && \
  make && \
  make install

  if [ -f "$WORKSPACE/ltmain.sh" ]; then
      rm "$WORKSPACE/ltmain.sh"
  fi

fi

# Install Aerospike Backup Service
if [ -n "$(find "$WORKSPACE/modules/aerospike-tools-backup" -mindepth 1 -print -quit)" ]; then
    make -C "$WORKSPACE/modules/aerospike-tools-backup" clean
else
    cd "$WORKSPACE" && git submodule update --init --recursive
fi
