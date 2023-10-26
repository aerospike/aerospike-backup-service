## Build local Docker images

### Create base image
```shell
docker build -t abs-base-image -f ./docker/Dockerfile.base .
```

### Build backup-service image for ARM64
```shell
docker build --build-arg GOARCH=arm64 -t backup-service -f ./docker/Dockerfile .
```

### Build backup-service image for AMD64
```shell
docker build --build-arg GOARCH=amd64 -t backup-service -f ./docker/Dockerfile .
```

## Build and push Docker images to JFrog

### Create and push base image
```shell
<DOCKER_USERNAME> <DOCKER_PASSWORD> ./docker/create-base-image.sh
```

### Create and push backup-service image with multi-platform support
```shell
TAG=<TAG> TAG_LATEST=false DOCKER_USERNAME=<DOCKER_USERNAME> DOCKER_PASSWORD=<DOCKER_PASSWORD> ./docker/create-abs-image.sh
```
