SOURCE_DIR:=$(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)

.PHONY: all
all: deb

.PHONY: deb
deb: install-manpage
	cd $(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/deb && \
	SOURCE_DIR=$(SOURCE_DIR) BINARY_NAME=$(BINARY_NAME) GIT_COMMIT=$(GIT_COMMIT) VERSION=$(VERSION) ARCH=$(ARCH) \
	dpkg-buildpackage -b -rfakeroot -us -uc -Zxz
	mv $(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/$(BINARY_NAME)_$(VERSION)-1_$(ARCH).deb \
    ../../target/$(BINARY_NAME)_$(VERSION)-1_$(ARCH).deb
	$(MAKE) clean

.PHONY: install-manpage
install-manpage:
	cp $(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/man/$(BINARY_NAME).1 \
	$(CURDIR)/$(ARCH)/$(BINARY_NAME)-$(VERSION)/packages/deb/debian

.PHONY: clean
clean:
	rm -rf $(ARCH)