.PHONY: all
all: rpm clean

.PHONY: rpm
rpm:
	rpmbuild -v -bb --define "_topdir $(CURDIR)" \
	--define "pkg_version $(VERSION)" \
	--define "pkg_name $(BINARY_NAME)" \
	--define "build_arch $(shell uname -m)" \
	--rmsource \
	--clean \
	$(BINARY_NAME).spec

.PHONY: clean
clean:
	unset GIT_COMMIT
	rm -rf BUILD BUILDROOT SOURCES RPMS SRPMS
