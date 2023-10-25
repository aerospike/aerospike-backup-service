To build the Docker images for local usage, follow these steps:

1. **Create a base image for local usage:**
    ```shell
    docker build -t abs-base-image -f ./dockerfiles/Dockerfile-base-image .
    ```

2. **Build the backup service image for ARM64:**
    ```shell
    docker build --build-arg GOARCH=arm64 -t backup-service -f ./dockerfiles/Dockerfile .
    ```

<br>

To build the Docker images for JFrog, follow these steps:
1. **Create the base image with multi-platform support:**
    ```shell
     <DOCKER_USERNAME> <DOCKER_PASSWORD> ./dockerfiles/create-multi-platform-base-image.sh
    ```
2. **Create the backup-service image with multi-platform support:**
    ```shell
     TAG=<TAG> TAG_LATEST=false DOCKER_USERNAME=<DOCKER_USERNAME> DOCKER_PASSWORD=<DOCKER_PASSWORD> ./dockerfiles/create-multi-platform-backup-service-image.sh
    ```
