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

ARCHS=linux/amd64 linux/arm64
PACKAGERS=deb rpm
TARGET=$(TARGET_DIR)/$(BINARY_NAME)
ifneq ($(strip $(OS))$(strip $(ARCH)),)
	TARGET=$(TARGET_DIR)/$(BINARY_NAME)_$(OS)_$(ARCH)
endif
GIT_COMMIT:=$(shell git rev-parse HEAD)
VERSION:=$(shell cat VERSION)

# Go parameters
GO ?= $(shell which go || echo "/usr/local/go/bin/go")
NFPM ?= $(shell which nfpm)
OS ?= $($(GO) env GOOS)
ARCH ?= $($(GO) env GOARCH)
GOBUILD = GOOS=$(OS) GOARCH=$(ARCH) $(GO) build \
-ldflags="-X main.commit=$(GIT_COMMIT) -X main.buildTime=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
GOTEST = $(GO) test
GOCLEAN = $(GO) clean
GOBIN_VERSION = $(shell $(GO) version 2>/dev/null)

.PHONY: prep-submodules
prep-submodules:
	git submodule update --init --recursive

.PHONY: remove-submodules
remove-submodules:
	git submodule foreach --recursive git clean -fd
	git submodule deinit --all -f

.PHONY: buildx
buildx:
	@for arch in $(ARCHS); do \
  		OS=$$(echo $$arch | cut -d/ -f1); \
  		ARCH=$$(echo $$arch | cut -d/ -f2); \
  		OS=$$OS ARCH=$$ARCH $(MAKE) build; \
  	done

.PHONY: build
build: prep-submodules
	mkdir -p $(TARGET_DIR)
	$(GOBUILD) -o $(TARGET) ./$(CMD_DIR)

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
			--packager $$(echo $$packager) \
			--target $(TARGET_DIR); \
			done; \
  	done; \
  	$(MAKE) checksums

.PHONY: checksums
checksums:
	@find . -type f \
		\( -name '*.deb' -o -name '*.rpm' \) \
		-exec sh -c 'sha256sum "$$1" | cut -d" " -f1 > "$$1.sha256"' _ {} \;

.PHONY: docker-build
docker-build:
	 docker build --tag aerospike/aerospike-backup-service:$(TAG) --file $(WORKSPACE)/Dockerfile .

.PHONY: docker-buildx
docker-buildx:
	cd ./build/scripts && ./docker-buildx.sh --tag $(TAG)

.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: release
release:
	cd ./build/scripts && ./release.sh $(NEXT_VERSION)

.PHONY: clean
clean: remove-submodules
	$(GOCLEAN)
	rm $(TARGET_DIR)/*
	@find . -type f -name 'nfpm-*-*.yaml' -exec rm -f {} +
