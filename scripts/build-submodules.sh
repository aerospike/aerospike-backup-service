WORKSPACE="$(pwd)"
ZSTD_STATIC_PATH=""
OPENSSL_STATIC_PATH=""
DYNAMIC=false
POSITIONAL_ARGS=()

while [[ $# -gt 0 ]]; do
  case $1 in
  --dynamic)
    DYNAMIC=true
    shift
    shift
    ;;
  -* | --*)
    echo "Unknown option $1"
    exit 1
    ;;
  *)
    POSITIONAL_ARGS+=("$1") # save positional arg
    shift                   # past argument
    ;;
  esac
done

set -- "${POSITIONAL_ARGS[@]}"

if [ "$(uname -s)" == "Darwin" ]; then
  if [ "$DYNAMIC" == "true" ]; then
    make -C "$WORKSPACE/modules/aerospike-tools-backup" shared \
    EVENT_LIB=libuv
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
    if [ "$(grep '^VERSION_ID' /etc/os-release | cut -d'=' -f2 | cut -d'.' -f1 | tr -d '"')" == 8 ]; then
      OPENSSL_STATIC_PATH="/opt/openssl/lib64"
    fi
    if [ "$(uname -m)" == "aarch64" ]; then
      OPENSSL_STATIC_PATH="/usr/local/lib"
      if [ "$(grep '^VERSION_ID' /etc/os-release | cut -d'=' -f2 | cut -d'.' -f1 | tr -d '"')" == 8 ]; then
        OPENSSL_STATIC_PATH="/opt/openssl/lib"
      fi
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
