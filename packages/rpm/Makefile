.PHONY: all
all: rpm mv

.PHONY: rpm
rpm:
	rpmbuild -v -bb \
	--define "_topdir $(CURDIR)" \
	--define "pkg_version $(VERSION)" \
	--define "pkg_name $(BINARY_NAME)" \
	--define "build_arch $(shell uname -m)" \
	--define "_builddir $(CURDIR)/BUILD/$(shell uname -m)" \
	--rmsource \
	--clean \
	$(BINARY_NAME).spec

.PHONY: mv
mv:
	cp ./RPMS/$(shell uname -m)/$(BINARY_NAME)-$(VERSION)-1.*.$(shell uname -m).rpm "../../target"

.PHONY: clean
clean:
	unset GIT_COMMIT BINARY_NAME VERSION ARCH
	rm -rf BUILD BUILDROOT SOURCES RPMS SRPMS
