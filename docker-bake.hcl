group "default" {
    targets = [
        "backup-service"
    ]
}

variable LATEST {
    default = false
}

variable TAG {
    default = ""
}

variable "HUB" {
    default = "aerospike.jfrog.io/ecosystem-container-dev-local"
}

variable "PREFIX" {
    default = "aerospike"
}

variable SETTINGS {
    default = ""
}

variable PLATFORMS {
    default = "linux/amd64,linux/arm64"
}

variable TAG_PLATFORM_SUFFIX {
    default = ""
}

function "tags" {
    params = [service]
    result = LATEST == true ? [
        "${HUB}/${PREFIX}-${service}:${TAG}${TAG_PLATFORM_SUFFIX}",
        "${HUB}/${PREFIX}-${service}:latest${TAG_PLATFORM_SUFFIX}"] : ["${HUB}/${PREFIX}-${service}:${TAG}${TAG_PLATFORM_SUFFIX}"]
}

target "backup-service" {
    dockerfile = "./Dockerfile"
    platforms  = split(",", "${PLATFORMS}")
    tags = tags("backup-service")

    output = ["type=image,push=true"]
}
