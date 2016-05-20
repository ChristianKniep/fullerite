FULLERITE      := fullerite
BEATIT         := beatit
VERSION        := 0.4.13
SRCDIR         := src
HANDLER_DIR    := $(SRCDIR)/fullerite/handler
PROTO_SFX      := $(HANDLER_DIR)/signalfx.proto
GEN_PROTO_SFX  := $(HANDLER_DIR)/signalfx.pb.go
PKGS           := \
	$(BEATIT) \
	$(FULLERITE) \
	$(FULLERITE)/collector \
	$(FULLERITE)/config \
	$(FULLERITE)/handler \
	$(FULLERITE)/internalserver \
	$(FULLERITE)/metric \
	$(FULLERITE)/util

SOURCES        := $(foreach pkg, $(PKGS), $(wildcard $(SRCDIR)/$(pkg)/*.go))
SOURCES        := $(filter-out $(GEN_PROTO_SFX), $(SOURCES))
OS	       := $(shell /usr/bin/lsb_release -si 2> /dev/null)

space :=
space +=
comma := ,

# symlinks confuse go tools, let's not mess with it and use -L
GOPATH  := $(shell pwd -L)
export GOPATH

PATH := bin:$(PATH)
export PATH

GO15VENDOREXPERIMENT := 0
export GO15VENDOREXPERIMENT

all: clean fmt lint $(FULLERITE) $(BEATIT) test

.PHONY: clean
clean:
	@echo Cleaning $(FULLERITE)...
	@rm -f $(FULLERITE) bin/$(FULLERITE)
	@rm -f $(BEATIT) bin/$(BEATIT)
	@rm -rf pkg/*/$(FULLERITE)
	@rm -rf build fullerite*.deb fullerite*.rpm
# Let's keep the generated file in the repo for ease of development.
#	@rm -f $(GEN_PROTO_SFX)

deps:
	@echo Getting dependencies...
	@go get github.com/mattn/gom
	@gom install > /dev/null

$(FULLERITE): $(SOURCES) deps
	@echo Building $(FULLERITE)...
	@gom build -o bin/$(FULLERITE) $@

$(BEATIT): $(BEATIT_SOURCES)
	@echo Building $(BEATIT)...
	@gom build -o bin/$(BEATIT) $@

test: tests
tests: deps diamond_core_test
	@echo Testing $(FULLERITE)
	@for pkg in $(PKGS); do \
		gom test -cover $$pkg || exit 1;\
	done

qbt:
	@echo Fast testing $(FULLERITE)
	@for pkg in $(PKGS); do \
		gom test -v -cover $$pkg || exit 1;\
	done

diamond_core_test:
	@python src/diamond/test.py -d

coverage_report: deps
	@echo Creating a coverage rport for $(FULLERITE)
	@$(foreach pkg, $(PKGS), gom test -coverprofile=coverage.out -coverpkg=$(subst $(space),$(comma),$(PKGS)) $(pkg);)
	@gom tool cover -html=coverage.out



fmt: deps $(SOURCES)
	@$(foreach pkg, $(PKGS), gom fmt $(pkg);)

vet: deps $(SOURCES)
	@echo Vetting $(FULLERITE) sources...
	@$(foreach pkg, $(PKGS), gom vet $(pkg);)

proto: protobuf
protobuf: deps $(PROTO_SFX)
	@echo Compiling protobuf
	@go get -u github.com/golang/protobuf/proto
	@go get -u github.com/golang/protobuf/protoc-gen-go
	@protoc --go_out=. $(PROTO_SFX)

lint: deps $(SOURCES)
	@echo Linting $(FULLERITE) sources...
	@$(foreach src, $(SOURCES), _vendor/bin/golint $(src);)

cyclo: deps $(SOURCES)
	@echo Checking code complexity...
	@_vendor/bin/gocyclo $(SOURCES)

pkg: package
package: clean $(FULLERITE) $(BEATIT)
	@echo Packaging for $(OS)
	@mkdir -p build/usr/bin build/usr/share/fullerite build/etc
	@cp bin/fullerite build/usr/bin/
	@cp bin/beatit build/usr/bin/
	@cp deb/bin/run-* build/usr/bin/
	@cp examples/config/fullerite.conf.example build/etc/
	@cp -r src/diamond build/usr/share/fullerite/diamond
ifeq ($(OS),Ubuntu)
	@fpm -s dir \
		-t deb \
		--name $(FULLERITE) \
		--version $(VERSION) \
		--description "metrics collector" \
		--depends python \
		--deb-user "fuller" \
		--deb-group "fuller" \
		--deb-default "deb/etc/fullerite" \
		--deb-upstart "deb/etc/init/fullerite" \
		--deb-upstart "deb/etc/init/fullerite_diamond_server" \
		--before-install "deb/before_install.sh" \
		--before-remove "deb/before_rm.sh" \
		--after-remove "deb/post_rm.sh" \
		-C build .
# CentOS 7 Only
else ifeq ($(OS),CentOS)
	@fpm -s dir \
		-t rpm \
		--name $(FULLERITE) \
		--version $(VERSION) \
		--description "metrics collector" \
		--depends python \
		--rpm-user "fuller" \
		--rpm-group "fuller" \
                --before-install "rpm/before_install.sh" \
		--before-remove "rpm/before_rm.sh" \
		-C build . \
		../rpm/fullerite.systemd=/etc/systemd/system/fullerite.service \
                ../rpm/fullerite.sysconfig=/etc/sysconfig/fullerite
else
	@echo "OS not supported"
endif
