SHELL = bash
WORKSPACE = $(shell pwd)
MAINTAINER = "Aerospike <info@aerospike.com>"
DESCRIPTION = "Aerospike Backup Service"
HOMEPAGE = "https://www.aerospike.com"
VENDOR = "Aerospike INC"
LICENSE = "Apache License 2.0"

BINARY_NAME=aerospike-backup-service
CMD_DIR = cmd/backup
BUILD_DIR = build
TARGET_DIR = $(BUILD_DIR)/target
PACKAGE_DIR = $(BUILD_DIR)/package

ARCHS ?= linux/amd64 linux/arm64
PACKAGERS ?= deb rpm

IMAGE_TAG ?= test
IMAGE_REPO ?= aerospike/aerospike-backup-service
IMAGE_CACHE_FROM ?=
IMAGE_CACHE_TO ?=
IMAGE_OUTPUT ?= type=image,push=true
TARGET=$(TARGET_DIR)/$(BINARY_NAME)
ifneq ($(strip $(OS))$(strip $(ARCH)),)
	TARGET=$(TARGET_DIR)/$(BINARY_NAME)_$(OS)_$(ARCH)
endif

GIT_COMMIT := $(shell git rev-parse HEAD)
VERSION ?= $(shell cat VERSION)

# Go parameters
GO ?= $(shell which go || echo "/usr/local/go/bin/go")
NFPM ?= $(shell which nfpm)
OS ?= $(shell $(GO) env GOOS)
ARCH ?= $(shell $(GO) env GOARCH)
REGISTRY ?= "docker.io"

GOBUILD = GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 $(GO) build \
-trimpath -ldflags="-s -w -X main.commitHash=$(GIT_COMMIT) -X main.buildTime=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
GOTEST = $(GO) test
GOCLEAN = $(GO) clean
GOBIN_VERSION = $(shell $(GO) version 2>/dev/null)

.PHONY: submodules
submodules:
	git submodule update --init --recursive

.PHONY: build
build: submodules
	mkdir -p $(TARGET_DIR)
	$(GOBUILD) -o $(TARGET) ./$(CMD_DIR)

.PHONY: buildx
buildx:
	@for arch in $(ARCHS); do \
		OS=$$(echo $$arch | cut -d/ -f1); \
		ARCH=$$(echo $$arch | cut -d/ -f2); \
		OS=$$OS ARCH=$$ARCH $(MAKE) build; \
	done

.PHONY: packages
packages: buildx
	@for arch in $(ARCHS); do \
		OS=$$(echo $$arch | cut -d/ -f1); \
		ARCH=$$(echo $$arch | cut -d/ -f2); \
		OS=$$OS ARCH=$$ARCH \
		NAME=$(BINARY_NAME) \
		VERSION=$(VERSION) \
		WORKSPACE=$(WORKSPACE) \
		MAINTAINER=$(MAINTAINER) \
		DESCRIPTION=$(DESCRIPTION) \
		HOMEPAGE=$(HOMEPAGE) \
		VENDOR=$(VENDOR) \
		LICENSE=$(LICENSE) \
		envsubst '$$OS $$ARCH $$NAME $$VERSION $$WORKSPACE $$MAINTAINER $$DESCRIPTION $$HOMEPAGE $$VENDOR $$LICENSE' \
		< $(PACKAGE_DIR)/nfpm.tmpl.yaml > $(PACKAGE_DIR)/nfpm-$$OS-$$ARCH.yaml; \
		for packager in $(PACKAGERS); do \
			$(NFPM) package \
			--config $(PACKAGE_DIR)/nfpm-$$OS-$$ARCH.yaml \
			--packager $$packager \
			--target $(TARGET_DIR); \
		done; \
	done

.PHONY: checksums
checksums:
	@find . -type f \
		\( -name '*.deb' -o -name '*.rpm' \) \
		-exec sh -c 'sha256sum "$$1" | cut -d" " -f1 > "$$1.sha256"' _ {} \;

.PHONY: docker-build
docker-build:
	DOCKER_BUILDKIT=1 docker build --progress=plain \
	--tag $(IMAGE_REPO):$(IMAGE_TAG) \
	--build-arg REGISTRY=$(REGISTRY) \
	--file $(WORKSPACE)/Dockerfile .

.PHONY: docker-buildx
docker-buildx:
	cd ./build/scripts && ./docker-buildx.sh \
	--repo $(IMAGE_REPO) \
	--tag $(IMAGE_TAG) \
	--registry $(REGISTRY) \
	--platforms "$(ARCHS)" \
	--cache-to "$(IMAGE_CACHE_TO)" \
	--cache-from "$(IMAGE_CACHE_FROM)" \
	--output "$(IMAGE_OUTPUT)"

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: release
release: service-release helm-chart-release

.PHONY: service-release
service-release:
	cd ./build/scripts && ./release.sh $(NEXT_VERSION)

.PHONY: helm-chart-release
helm-chart-release:
	cd ./build/scripts && ./helm-chart-release.sh $(NEXT_HELM_CHART_VERSION)

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(TARGET_DIR)/*
	@find . -type f -name 'nfpm-*-*.yaml' -exec rm -f {} +
	git submodule foreach --recursive git clean -fd; \
	git submodule deinit --all -f

.PHONY: vulnerability-scan
vulnerability-scan:
	snyk test --policy-path=.snyk --severity-threshold=high

.PHONY: vulnerability-scan-container
vulnerability-scan-container:
	snyk container test $(IMAGE_REPO):$(IMAGE_TAG) \
	--policy-path=.snyk \
	--file=Dockerfile \
	--severity-threshold=high
