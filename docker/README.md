## Build Docker image in stages

### Create base image
```shell
docker build -t aerospike-backup-service-base -f ./docker/Dockerfile.base .
```

### Build backup-service image for ARM64
```shell
docker build --build-arg GOARCH=arm64 -t backup-service -f ./docker/Dockerfile .
```

### Build backup-service image for AMD64
```shell
docker build --build-arg GOARCH=amd64 -t backup-service -f ./docker/Dockerfile .
```
