SHELL = bash

WORKSPACE = $(shell pwd)
UNAME = $(shell uname -sm | tr ' ' '-')
UNAME_M=$(shell uname -m)

BINARY_NAME=aerospike-backup-service
GIT_TAG = $(shell git describe --tags)
CMD_DIR = cmd/backup
TARGET_DIR = target
PACKAGES_DIR = packages
LIB_DIR = lib
PKG_DIR = build/package
PREP_DIR = $(TARGET_DIR)/pkg_install
CONFIG_FILES = $(wildcard config/*)
POST_INSTALL_SCRIPT = $(PKG_DIR)/post-install.sh
TOOLS_DIR = $(WORKSPACE)/modules/aerospike-tools-backup
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
GO_VERSION = 1.22.5
GOBIN_VERSION = $(shell $(GO) version 2>/dev/null)


MAINTAINER = "Aerospike"
DESCRIPTION = "Aerospike Backup Service"
URL = "https://www.aerospike.com"
VENDOR = "Aerospike"
LICENSE = "Apache License 2.0"

.PHONY: install-deps
install-deps:
	./scripts/install-deps.sh

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
build:
	mkdir -p $(TARGET_DIR)
	$(GOBUILD) -o $(TARGET) ./$(CMD_DIR)

.PHONY: packages
packages: buildx
	@for arch in $(ARCHS); do \
  		OS=$$(echo $$arch | cut -d/ -f1); \
  		ARCH=$$(echo $$arch | cut -d/ -f2); \
		OS=$$OS ARCH=$$ARCH \
		NAME=$(BINARY_NAME) VERSION=$(VERSION) WORKSPACE=$(WORKSPACE) \
		envsubst '$$OS $$ARCH $$NAME $$VERSION $$WORKSPACE' \
		< $(PACKAGES_DIR)/nfpm.tmpl.yaml > $(PACKAGES_DIR)/nfpm-$$OS-$$ARCH.yaml; \
		for packager in $(PACKAGERS); do \
			$(NFPM) package \
			--config $(PACKAGES_DIR)/nfpm-$$OS-$$ARCH.yaml \
			--packager $$(echo $$packager) \
			--target $(TARGET_DIR); \
			done; \
  	done
.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: rpm
rpm: tarball
	mkdir -p $(WORKSPACE)/target
	mkdir -p $(WORKSPACE)/packages/rpm/SOURCES
	mv /tmp/$(BINARY_NAME)-$(VERSION)-$(UNAME_M).tar.gz $(WORKSPACE)/packages/rpm/SOURCES/
	BINARY_NAME=$(BINARY_NAME) GIT_COMMIT=$(GIT_COMMIT) VERSION=$(VERSION) $(MAKE) -C packages/rpm

.PHONY: deb
deb: tarball
	mkdir -p $(WORKSPACE)/target
	mkdir -p $(WORKSPACE)/packages/deb/$(ARCH)
	tar -xvf /tmp/$(BINARY_NAME)-$(VERSION)-$(UNAME_M).tar.gz -C $(WORKSPACE)/packages/deb/$(ARCH)
	BINARY_NAME=$(BINARY_NAME) GIT_COMMIT=$(GIT_COMMIT) VERSION=$(VERSION) ARCH=$(ARCH) $(MAKE) -C packages/deb

.PHONY: tarball
tarball: prep-submodules
	cd ./scripts && ./tarball.sh

.PHONY: release
release:
	cd ./scripts && ./release.sh $(NEXT_VERSION)

.PHONY: clean
clean:
	$(GOCLEAN)
	$(MAKE) clean-submodules
	rm -rf $(TARGET_DIR) $(LIB_DIR)
