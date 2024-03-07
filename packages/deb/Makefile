SOURCE_DIR:=$(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)

.PHONY: all
all: deb

.PHONY: deb
deb: gen-substvars install-manpage
	cd $(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/deb && \
	SOURCE_DIR=$(SOURCE_DIR) BINARY_NAME=$(BINARY_NAME) GIT_COMMIT=$(GIT_COMMIT) VERSION=$(VERSION) ARCH=$(ARCH) dpkg-buildpackage --no-sign


.PHONY: gen-substvars
gen-substvars:
	@echo "abs:version=$(VERSION)" > $(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/deb/debian/substvars

.PHONY: install-manpage
install-manpage:
	cp $(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/man/$(BINARY_NAME).1 \
	$(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/deb/debian



#.PHONY: mv
#mv:
#	cp ./RPMS/$(shell uname -m)/$(BINARY_NAME)-$(VERSION)-1.*.$(shell uname -m).rpm "../../target"

.PHONY: clean
clean:
	unset GIT_COMMIT BINARY_NAME VERSION ARCH SOURCE_DIR
	#rm -rf BUILD BUILDROOT SOURCES RPMS SRPMS