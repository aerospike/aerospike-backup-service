WORKSPACE="$(pwd)"
ZSTD_STATIC_PATH=""
OPENSSL_STATIC_PATH=""

if [ "$(uname -o)" == "Darwin" ]; then
  if [ "$(uname -m)" == "x86_64" ]; then
    make -C "$WORKSPACE/modules/aerospike-tools-backup" shared \
    EVENT_LIB=libuv \
    OPENSSL_STATIC_PATH="$(brew --prefix openssl@1.1)/lib"
  else
    make -C "$WORKSPACE/modules/aerospike-tools-backup" shared \
    EVENT_LIB=libuv \
    OPENSSL_STATIC_PATH="$(brew --prefix openssl@1.1)/lib"
  fi

else
  DISTRO="$(cat /etc/os-release | grep ^ID_LIKE= /etc/os-release | cut -d'=' -f2)"
  if [ "$DISTRO" == "debian" ]; then
    ZSTD_STATIC_PATH="/usr/local/lib"
    OPENSSL_STATIC_PATH="/usr/local/lib64"
    if [ "$(uname -m)" == "aarch64" ]; then
      OPENSSL_STATIC_PATH="/usr/local/lib"
    fi
  else
    ZSTD_STATIC_PATH="/usr/local/lib64"
    OPENSSL_STATIC_PATH="/usr/local/lib64"
    if [ "$(uname -m)" == "aarch64" ]; then
      OPENSSL_STATIC_PATH="/usr/local/lib"
    fi
  fi
  make -C "$WORKSPACE/modules/aerospike-tools-backup" shared \
  EVENT_LIB=libuv \
  AWS_SDK_STATIC_PATH=/usr/local/lib \
  CURL_STATIC_PATH=/usr/local/lib \
  ZSTD_STATIC_PATH="$ZSTD_STATIC_PATH" \
  OPENSSL_STATIC_PATH="$OPENSSL_STATIC_PATH" \
  LIBUV_STATIC_PATH=/usr/local/lib \
  JANSSON_STATIC_PATH=/usr/local/lib
fi