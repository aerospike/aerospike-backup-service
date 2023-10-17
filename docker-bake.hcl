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

function "tags" {
    params = [service]
    result = LATEST == true ? [
        "${HUB}/${PREFIX}-${service}:${TAG}",
        "${HUB}/${PREFIX}-${service}:latest"] : ["${HUB}/${PREFIX}-${service}:${TAG}"]
}

target "backup-service" {
    dockerfile = "./Dockerfile"
    platforms  = split(",", "${PLATFORMS}")
    tags = tags("backup-service")

    output = ["type=image,push=true"]
}
