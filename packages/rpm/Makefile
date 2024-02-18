MOCK_CONFIG?=default
RESULT_DIR?="../../target"

.PHONY: all
all: rpm

.PHONY: SOURCES
SOURCES:
	mkdir -p "$(CURDIR)/SOURCES"

.PHONY: archive
archive: SOURCES
	cd ../../ && \
	git archive --prefix=$(BINARY_NAME)-$(VERSION)/ \
		-o "$(CURDIR)/SOURCES/$(BINARY_NAME)-$(VERSION).tar.gz" HEAD

.PHONY: build_prepare
build_prepare: archive
	rm -f $(RESULT_DIR)/$(BINARY_NAME)*.rpm

.PHONY: rpm
rpm: build_prepare
	rpmbuild -v -bb --define "pkg_version $(VERSION)" \
	--define "pkg_name $(BINARY_NAME)" \
	--define "build_arch $(shell uname -m)" \
	--define "build_arch $(shell uname -m)" \
	$(BINARY_NAME).spec
	

	@echo "VERSION: $(VERSION)"
	@echo "RESULT_DIR: $(RESULT_DIR)"
	@echo "BINARY_NAME: $(BINARY_NAME)"
	@echo "PWD: $(PWD)"
	@echo "CURDIR: $(CURDIR)"



#srpm: build_prepare
#	/usr/bin/mock \
#		-r $(MOCK_CONFIG) \
#		--define "__version $(VERSION)" \
#		--define "__release $(BUILD_NUMBER)" \
#		--resultdir=$(RESULT_DIR) \
#		--buildsrpm \
#		--spec=librdkafka.spec \
#		--sources=SOURCES
#	@echo "======= Source RPM now available in $(RESULT_DIR) ======="
#
#rpm: srpm
#	/usr/bin/mock \
#		-r $(MOCK_CONFIG) \
#		--define "__version $(VERSION)"\
#		--define "__release $(BUILD_NUMBER)"\
#		--resultdir=$(RESULT_DIR) \
#		--rebuild $(RESULT_DIR)/$(BINARY_NAME)*.src.rpm
#	@echo "======= Binary RPMs now available in $(RESULT_DIR) ======="

#clean:
#	rm -rf SOURCES
#
#distclean: clean
#	rm -f build.log root.log state.log available_pkgs installed_pkgs \
#		*.rpm *.tar.gz