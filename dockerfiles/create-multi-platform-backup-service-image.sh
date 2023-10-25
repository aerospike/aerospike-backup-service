docker buildx create --use --name mybuilder
docker login aerospike.jfrog.io -u "$DOCKER_USERNAME" -p "$DOCKER_PASSWORD"
docker buildx build -t aerospike.jfrog.io/ecosystem-container-dev-local/aerospike-backup-service:fast --push --platform linux/amd64,linux/arm64 \
--build-arg BASE_IMAGE=aerospike.jfrog.io/ecosystem-container-dev-local/abs-base-image:latest \
 --file ./dockerfiles/Dockerfile .
docker buildx rm mybuilder
