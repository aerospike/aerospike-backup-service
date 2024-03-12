SHELL = bash

WORKSPACE = $(shell pwd)
UNAME = $(shell uname -sm | tr ' ' '-')
UNAME_M=$(shell uname -m)

ifeq ($(UNAME_M),x86_64)
    ARCH:=amd64
else ifeq ($(UNAME_M),aarch64)
    ARCH:=arm64
else
    $(error Unsupported architecture)
endif

BINARY_NAME:=aerospike-backup-service
GIT_COMMIT:=$(shell git rev-parse HEAD)
VERSION:=$(shell cat VERSION)

# Go parameters
GO ?= $(shell which go || echo "/usr/local/go/bin/go")
CGO_CFLAGS=-I $(WORKSPACE)/modules/aerospike-tools-backup/modules/c-client/target/$(UNAME)/include \
-I $(WORKSPACE)/modules/aerospike-tools-backup/modules/secret-agent-client/target/$(UNAME)/include \
-I $(WORKSPACE)/modules/aerospike-tools-backup/include
GOBUILD = CGO_CFLAGS="$(CGO_CFLAGS)" CGO_ENABLED=1 $(GO) build \
-ldflags="-X main.commit=$(GIT_COMMIT) -X main.buildTime=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
GOTEST = $(GO) test
GOCLEAN = $(GO) clean
GO_VERSION = 1.22.0
GOBIN_VERSION = $(shell $(GO) version 2>/dev/null)


GIT_TAG = $(shell git describe --tags)
CMD_DIR = cmd/backup
TARGET_DIR = target
LIB_DIR = lib
PKG_DIR = build/package
PREP_DIR = $(TARGET_DIR)/pkg_install
CONFIG_FILES = $(wildcard config/*)
POST_INSTALL_SCRIPT = $(PKG_DIR)/post-install.sh
TOOLS_DIR = $(WORKSPACE)/modules/aerospike-tools-backup

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

.PHONY: build-submodules
build-submodules:
	./scripts/build-submodules.sh
	./scripts/copy_shared.sh

.PHONY: clean-submodules
clean-submodules:
	$(MAKE) -C $(TOOLS_DIR) clean

.PHONY: build
build:
	mkdir -p $(TARGET_DIR)
	$(GOBUILD) -o $(TARGET_DIR)/$(BINARY_NAME) ./$(CMD_DIR)
.PHONY: test
test:
	$(GOTEST) -v ./...

.PHONY: rpm
rpm: tarball
	mkdir -p $(WORKSPACE)/packages/rpm/SOURCES
	mv /tmp/$(BINARY_NAME)-$(VERSION)-$(UNAME_M).tar.gz $(WORKSPACE)/packages/rpm/SOURCES/
	BINARY_NAME=$(BINARY_NAME) GIT_COMMIT=$(GIT_COMMIT) VERSION=$(VERSION) $(MAKE) -C packages/rpm

.PHONY: deb
deb: tarball
	mkdir -p $(WORKSPACE)/packages/deb/$(ARCH)
	tar -xvf /tmp/$(BINARY_NAME)-$(VERSION)-$(UNAME_M).tar.gz -C $(WORKSPACE)/packages/deb/$(ARCH)
	BINARY_NAME=$(BINARY_NAME) GIT_COMMIT=$(GIT_COMMIT) VERSION=$(VERSION) ARCH=$(ARCH) $(MAKE) -C packages/deb

.PHONY: tarball
tarball: prep-submodules
	cd ./scripts && ./tarball.sh

.PHONY: clean
clean:
	$(GOCLEAN)
	$(MAKE) clean-submodules
	rm -rf $(TARGET_DIR) $(LIB_DIR)
