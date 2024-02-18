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
	mv $(CURDIR)/RPMS/$(shell uname -m)/*.rpm $(PWD)/target

clean:
	rm -rf BUILD BUILDROOT SOURCES RPMS SRPMS
