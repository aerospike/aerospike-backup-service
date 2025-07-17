group default {
  targets = [
    "aerospike-backup-service"
  ]
}

variable CONTEXT {
  default = null
}

variable LATEST {
  default = false
}

variable TAG {
  default = ""
}

variable GIT_BRANCH {
  default = null
}

variable GIT_COMMIT_SHA {
  default = null
}

variable VERSION {
  default = null
}

variable ISO8601 {
  default = null
}

variable REPO {
  default = "aerospike/aerospike-backup-service"
}

variable PLATFORMS {
  default = "linux/amd64,linux/arm64"
}

variable REGISTRY {
  default = "docker.io"
}

variable GO_VERSION {
  default = "1.23.4"
}

variable CACHE_FROM {
  default = ""
}

variable CACHE_TO {
  default = ""
}

variable OUTPUT {
  default = "type=image,push=true"
}

function norm {
  params = [value]

  result = value == null || value == "" ? [] : length(regexall(" ", value)) > 0 ? split(" ", value) : [value]
}

function tags {
  params = [service]
  result = LATEST == true ? [
    "${REPO}/${service}:${TAG}",
    "${REPO}/${service}:latest"
  ] : ["${REPO}/${service}:${TAG}"]
}

target aerospike-backup-service {
  labels = {
    "org.opencontainers.image.title"         = "Aerospike Backup Service"
    "org.opencontainers.image.description"   = "Aerospike Backup Service provides a set of REST APIs to schedule full and incremental backups. Additionally, these APIs can be used to restore data from a backup to a cluster"
    "org.opencontainers.image.documentation" = "https://github.com/aerospike/aerospike-backup-service?tab=readme-ov-file#aerospike-backup-service"
    "org.opencontainers.image.base.name"     = "registry.access.redhat.com/ubi9/ubi-minimal"
    "org.opencontainers.image.source"        = "https://github.com/aerospike/aerospike-backup-service/tree/${GIT_BRANCH}"
    "org.opencontainers.image.vendor"        = "Aerospike"
    "org.opencontainers.image.version"       = "${VERSION}"
    "org.opencontainers.image.url"           = "https://github.com/aerospike/aerospike-backup-service"
    "org.opencontainers.image.licenses"      = "Apache-2.0"
    "org.opencontainers.image.revision"      = "${GIT_COMMIT_SHA}"
    "org.opencontainers.image.created"       = "${ISO8601}"
  }

  args = {
    GO_VERSION = "${GO_VERSION}"
    REGISTRY = "${REGISTRY}"
  }

  secret = [
    "id=GOPROXY,env=GOPROXY"
  ]
  context    = "${CONTEXT}"
  dockerfile = "Dockerfile"
  platforms = split(",", replace("${PLATFORMS}", " ", ","))
  cache-to = norm("${CACHE_TO}")
  cache-from = norm("${CACHE_FROM}")

  tags = tags("aerospike-backup-service")
  output = norm("${OUTPUT}")
}
