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

PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
DESTDIR ?=

GOBUILD = GOOS=$(OS) GOARCH=$(ARCH) CGO_ENABLED=0 $(GO) build \
-trimpath -ldflags="-s -w \
-X github.com/aerospike/aerospike-backup-service/v3.CommitHash=$(GIT_COMMIT) \
-X github.com/aerospike/aerospike-backup-service/v3.BuildTime=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"
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

.PHONY: install
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(TARGET) $(DESTDIR)$(BINDIR)/$(BINARY_NAME)

.PHONY: uninstall
uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)

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
	@GO_VERSION="$$(curl -s 'https://go.dev/dl/?mode=json&include=all' | \
		jq -r --arg ver "go$$($(GO) mod edit -json | \
		jq -r '.Go' | cut -d. -f1-2)" '.[] | select(.version | startswith($$ver)) | .version' | \
		sort -V | tail -n1 | cut -c3- | tr -d '\n')"; \
	DOCKER_BUILDKIT=1 docker build --progress=plain \
	--tag $(IMAGE_REPO):$(IMAGE_TAG) \
	--build-arg GO_VERSION="$$GO_VERSION" \
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

# mocks-generate: runs mockgen over pkg/service* interfaces and writes mockgen.go next to each
#   package. Used by tests; committed mocks must match this output (see mocks-check).
.PHONY: mocks-generate
mocks-generate:
	$(WORKSPACE)/build/scripts/generate-mocks.sh

# Ensure committed mock files match the output of mocks-generate (no hand-edits, no stale mocks).
# The find runs inside the recipe (not at parse time) so it picks up newly created files.
.PHONY: mocks-check
mocks-check: mocks-generate
	@UNTRACKED=$$(git ls-files --others --exclude-standard '*.mockgen.go' '**/mockgen.go'); \
	if [ -n "$$UNTRACKED" ]; then \
		echo "Untracked mock files found — these should be committed:"; \
		echo "$$UNTRACKED"; \
		exit 1; \
	fi
	@git diff --exit-code -- $$(find . -name 'mockgen.go' -not -path './.git/*') \
		|| (echo "Mock files are out of date. Run 'make mocks-generate' and commit the changes." && exit 1)

.PHONY: format
format:
	gci -w .
	$(GO) fmt ./...

.PHONY: lint
lint:
	golangci-lint run ./...

.PHONY: lint-fix
lint-fix:
	golangci-lint run --fix ./...

# openapi: runs swag over handlers/DTOs, then swagger2openapi, producing docs/openapi.json and
#   docs/config.schema.json. Requires Docker and Node (npx). Used by readme and API docs; not run in CI.
.PHONY: openapi
openapi:
	$(WORKSPACE)/build/scripts/generate-openapi.sh

# readme: runs build/readme (Go). Reads docs/openapi.json, writes README.md sections, docs/examples/*,
#   docs/readme/dto/*, and docs/metrics.json. Committed files must match (see readme-check).
.PHONY: readme
readme:
	$(GO) run ./build/readme

.PHONY: readme-check
readme-check: readme
	@git diff --exit-code -- README.md docs/examples/ docs/readme/dto/ docs/metrics.json \
		|| (echo "README / examples / docs are out of date. Run 'make readme' and commit the changes." && exit 1)
	@UNTRACKED=$$(git ls-files --others --exclude-standard 'docs/examples/' 'docs/readme/dto/' 'docs/metrics.json'); \
	if [ -n "$$UNTRACKED" ]; then \
		echo "Untracked generated doc files found — these should be committed:"; \
		echo "$$UNTRACKED"; \
		exit 1; \
	fi

.PHONY: tidy
tidy:
	$(GO) mod tidy

# Verify generated artifacts are committed and up to date (for CI).
.PHONY: generated-check
generated-check: mocks-check readme-check

# Full local PR checklist.
.PHONY: pr
pr: tidy mocks-check format lint-fix test openapi readme

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
