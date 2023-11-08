# Go parameters
GOCMD = go
UNAME = $(shell uname -sm | tr ' ' '-')
CGO_CFLAGS = -I/app/modules/aerospike-tools-backup/modules/c-client/target/$(UNAME)/include \
  -I/app/modules/aerospike-tools-backup/include
GOBUILD = CGO_CFLAGS="$(CGO_CFLAGS)" CGO_ENABLED=1 $(GOCMD) build
GOTEST = $(GOCMD) test
GOCLEAN = $(GOCMD) clean

LSB_EXISTS := $(shell which lsb_release 2> /dev/null)
ifeq ($(LSB_EXISTS),)
	DISTRO_FULL := $(shell . /etc/os-release 2> /dev/null; echo $$NAME | tr ' ' '_')
	DISTRO_VERSION := $(shell . /etc/os-release 2> /dev/null; echo $$VERSION_ID | tr ' ' '_')
else
	DISTRO_FULL := $(shell lsb_release -i | cut -f2- | tr ' ' '_')
	DISTRO_VERSION := $(shell lsb_release -r | cut -f2- | tr ' ' '_')
endif

ifeq ($(DISTRO_FULL),Debian)
	DISTRO_SHORT = debian
else ifeq ($(DISTRO_FULL),Ubuntu)
	DISTRO_SHORT = ubuntu
else ifeq ($(DISTRO_FULL),Amazon_Linux)
	DISTRO_SHORT = amzn
else ifeq ($(DISTRO_FULL),CentOS_Linux)
	DISTRO_SHORT = el
else ifeq ($(DISTRO_FULL),Red_Hat_Enterprise_Linux)
	DISTRO_SHORT = el
else ifeq ($(DISTRO_FULL),Rocky_Linux)
	DISTRO_SHORT = rocky
endif

BINARY_NAME = aerospike-backup-service
GIT_TAG = $(shell git describe --tags)

CMD_DIR = cmd/backup
TARGET_DIR = target
PKG_DIR = build/package
PREP_DIR = $(TARGET_DIR)/pkg_install
CONFIG_FILES = $(wildcard config/*)
POST_INSTALL_SCRIPT = $(PKG_DIR)/post-install.sh
TOOLS_DIR = modules/aerospike-tools-backup

MAINTAINER = "Aerospike"
DESCRIPTION = "Aerospike Backup Service"
URL = "https://www.aerospike.com"
VENDOR = "Aerospike"
LICENSE = "Apache License 2.0"

FPM_COMMON_ARGS = \
	--force \
	--input-type dir \
	--name $(BINARY_NAME) \
	--version $(GIT_TAG) \
	--chdir $(PREP_DIR) \
	--maintainer $(MAINTAINER) \
	--description $(DESCRIPTION) \
	--url $(URL) \
	--vendor $(VENDOR) \
	--license $(LICENSE) \
	--after-install $(POST_INSTALL_SCRIPT)

.PHONY: build-submodules
build-submodules:
	git submodule update --init --recursive
	$(MAKE) -C $(TOOLS_DIR) shared EVENT_LIB=libuv
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
package: rpm deb tar

.PHONY: rpm
rpm: build prep
	$(eval ARCH := $(shell uname -m))
	$(eval DISTRO_VERSION := $(shell echo $(DISTRO_VERSION) | cut -d'.' -f1)) # Only major version for RPM
	fpm $(FPM_COMMON_ARGS) \
		--output-type rpm \
		--package $(TARGET_DIR)/$(BINARY_NAME)-$(GIT_TAG)-1.$(DISTRO_SHORT)$(DISTRO_VERSION).$(ARCH).rpm

.PHONY: deb
deb: build prep
	@which dpkg-architecture > /dev/null || (echo "dpkg-architecture is not installed"; exit 1)
	$(eval ARCH := $(shell dpkg-architecture -q DEB_BUILD_ARCH))
	fpm $(FPM_COMMON_ARGS) \
		--output-type deb \
		--package $(TARGET_DIR)/$(BINARY_NAME)_$(GIT_TAG)-1$(DISTRO_SHORT)$(DISTRO_VERSION)_$(ARCH).deb

.PHONY: tar
tar: build prep
	fpm $(FPM_COMMON_ARGS) \
		--output-type tar \
		--package $(TARGET_DIR)/$(BINARY_NAME)_$(GIT_TAG)_$(DISTRO_SHORT)$(DISTRO_VERSION)_$(ARCH).tgz

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

.PHONY: clean
clean:
	$(GOCLEAN)
	rm -rf $(TARGET_DIR)

.PHONY: all
all: build test package