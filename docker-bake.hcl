group default {
  targets = [
    "aerospike-backup-service"
  ]
}

group local {
  targets = [
    "aerospike-backup-service-local"
  ]
}

variable CONTEXT {
  default = null
}

variable GITHUB_TOKEN {
  default = null
}

variable PROJECT {
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

variable HUB {
  default = "aerospike.jfrog.io/ecosystem-container-dev-local"
}

variable PLATFORMS {
  default = "linux/amd64,linux/arm64"
}

function tags {
  params = [service]
  result = LATEST == true ? [
    "${HUB}/${service}:${TAG}",
    "${HUB}/${service}:latest"
  ] : ["${HUB}/${service}:${TAG}"]
}

target aerospike-backup-service-local {
  args = {
    project = "${PROJECT}"
  }

  secret = [
    "id=GITHUB_TOKEN,env=GITHUB_TOKEN",
  ]
  context    = "${CONTEXT}"
  dockerfile = "Dockerfile"
  tags = tags("aerospike-backup-service")
  output = ["type=image"]
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
    project = "${PROJECT}"
  }

  secret = [
    "id=GITHUB_TOKEN,env=GITHUB_TOKEN",
  ]

  context    = "${CONTEXT}"
  dockerfile = "Dockerfile"
  platforms = split(",", "${PLATFORMS}")

  tags = tags("aerospike-backup-service")
  output = ["type=image,push=true"]
}
