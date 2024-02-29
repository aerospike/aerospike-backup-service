SHELL = bash

WORKSPACE = $(shell pwd)
UNAME = $(shell uname -sm | tr ' ' '-')
UNAME_M=$(shell uname -m)

ifeq ($(UNAME_M),x86_64)
    ARCH := amd64
else ifeq ($(UNAME_M),aarch64)
    ARCH := arm64
else
    $(error Unsupported architecture)
endif

export BINARY_NAME:=aerospike-backup-service
export GIT_COMMIT:=$(shell git rev-parse HEAD)
export VERSION:=$(shell cat VERSION)

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

.PHONY: package
package: rpm deb tarball

.PHONY: rpm
rpm: tarball
	mkdir -p $(WORKSPACE)/packages/rpm/SOURCES
	mv /tmp/$(BINARY_NAME)-$(VERSION).tar.gz $(WORKSPACE)/packages/rpm/SOURCES/
	$(MAKE) -C packages/rpm

.PHONY: deb
deb:
	echo "abs:version=$(VERSION)" > packages/debian/substvars
	cd $(WORKSPACE)/packages && dpkg-buildpackage
	mv $(WORKSPACE)/$(BINARY_NAME)_$(VERSION)-1_$(ARCH).deb $(WORKSPACE)/target
	$(MAKE) clean-deb
	rm -f $(WORKSPACE)/$(BINARY_NAME)_$(VERSION)-1_$(ARCH).*
	rm -f $(WORKSPACE)/$(BINARY_NAME)-dbgsym_$(VERSION)-1_$(ARCH).*
	rm -f $(WORKSPACE)/$(BINARY_NAME)_$(VERSION)-1.*
	rm -f $(WORKSPACE)/packages/debian/*.log
	rm -f $(WORKSPACE)/packages/debian/*.debhelper
	rm -f $(WORKSPACE)/packages/debian/*.substvars
	rm -f $(WORKSPACE)/packages/debian/debhelper-build-stamp
	rm -f $(WORKSPACE)/packages/debian/files
	rm -rf $(WORKSPACE)/packages/debian/$(BINARY_NAME)
	rm -rf $(WORKSPACE)/packages/debian/.debhelper
	$(MAKE) clean-submodules

.PHONY: install
install:
	@if [ "$$(uname)" == "Darwin" ]; then \
		echo "Running macOS specific commands"; \
	else \
		echo "Running Linux specific commands"; \
		# Add Linux specific commands here
	fi

.PHONY: prep
prep:
ifndef DISTRO_FULL
	$(error Distro not found)
endif

ifndef DISTRO_VERSION
	$(error Distro Version not found)
endif

	@echo "Distro: $(DISTRO_FULL)"
	@echo "Distro Version: $(DISTRO_VERSION)"

	@which git > /dev/null || (echo "Git is not installed"; exit 1)
	@which fpm > /dev/null || (echo "FPM is not installed"; exit 1)

	install -d $(PREP_DIR)
	install -d $(PREP_DIR)/usr/local/bin
	install -d $(PREP_DIR)/var/log/aerospike
	install -d $(PREP_DIR)/etc/$(BINARY_NAME)
	install -d $(PREP_DIR)/etc/systemd/system
	install -m 755 $(TARGET_DIR)/$(BINARY_NAME) $(PREP_DIR)/usr/local/bin/$(BINARY_NAME)
	install -m 644 $(CONFIG_FILES) $(PREP_DIR)/etc/$(BINARY_NAME)/
	install -m 644 $(PKG_DIR)/$(BINARY_NAME).service $(PREP_DIR)/etc/systemd/system/$(BINARY_NAME).service

.PHONY: tarball
tarball: prep-submodules
	cd ./scripts && ./tarball.sh

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(TARGET_DIR)

.PHONY: clean-deb
	cd $(WORKSPACE)/packages && dpkg-buildpackage -Tclean

.PHONY: clean-rpm
clean-rpm:
	rpmbuild --clean $(WORKSPACE)/packages/rpm/SPECS/$(BINARY_NAME).spec
	rm -rf $(WORKSPACE)/packages/rpm/SOURCES/*.tar.gz
	rm -rf $(WORKSPACE)/packages/rpm/SRPMS/*.rpm

.PHONY: process-submodules
process-submodules:
	git submodule foreach --recursive | while read -r submodule_path; do \
	echo "Processing submodule at path: $($$submodule_path | awk -F\' '{print $$2}')"; \
	done \

.PHONY: all
all: build test package
