export SOURCE_DIR:=$(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)

.PHONY: all
all: deb

.PHONY: deb
deb: gen-files
	dpkg-buildpackage

.PHONY: gen-files
gen-files:
	@echo "../$(ARCH)/target/$(BINARY_NAME) /usr/bin" > $(CURDIR)/debian/install
	@echo "../$(ARCH)/lib/* /usr/lib" >> $(CURDIR)/debian/install
	@echo "../$(ARCH)/packages/config/$(BINARY_NAME).yml /etc/$(BINARY_NAME)" >> $(CURDIR)/debian/install
	@echo "abs:version=$(VERSION)" > $(CURDIR)/debian/substvars

#.PHONY: mv
#mv:
#	cp ./RPMS/$(shell uname -m)/$(BINARY_NAME)-$(VERSION)-1.*.$(shell uname -m).rpm "../../target"

.PHONY: clean
clean:
	unset GIT_COMMIT BINARY_NAME VERSION ARCH SOURCE_DIR
	#rm -rf BUILD BUILDROOT SOURCES RPMS SRPMS