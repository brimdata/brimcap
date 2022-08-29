ARCH = amd64
VERSION = $(shell git describe --tags --dirty --always)
LDFLAGS = -s -X github.com/brimdata/brimcap/cli.Version=$(VERSION)

SURICATATAG = v5.0.3-brim3
SURICATAPATH = suricata-$(SURICATATAG)
ZEEKTAG = v3.2.1-brim10
ZEEKPATH = zeek-$(ZEEKTAG)

ZIP = zip -r
ifeq ($(shell go env GOOS),windows)
	ZIP=7z a
endif

# This enables a shortcut to run a single ztest e.g.:
#  make TEST=TestBrimpcap/cmd/brimcap/ztests/analyze-all
ifneq "$(TEST)" ""
test-one: test-run
endif

.PHONY: fmt
fmt:
	@gofmt -s -w .
	@git diff --exit-code

.PHONY: tidy
tidy:
	@go mod tidy
	@git diff --exit-code -- go.mod go.sum

build/$(ZEEKPATH).zip:
	@mkdir -p build
	@curl -L -o $@ \
		https://github.com/brimdata/zeek/releases/download/$(ZEEKTAG)/zeek-$(ZEEKTAG).$$(go env GOOS)-$(ARCH).zip

build/$(SURICATAPATH).zip:
	@mkdir -p build
	@curl -L -o $@ \
		https://github.com/brimdata/build-suricata/releases/download/$(SURICATATAG)/suricata-$(SURICATATAG).$$(go env GOOS)-$(ARCH).zip

build/dist/zeek: build/$(ZEEKPATH).zip
	@mkdir -p dist
	@unzip -q $^ -d build/dist
	@touch $@

build/dist/suricata: build/$(SURICATAPATH).zip
	@mkdir -p dist
	@unzip -q $^ -d build/dist
	@touch $@

bin/zq: go.mod
	@GOBIN="$(CURDIR)/bin" go install \
		github.com/brimdata/zed/cmd/zq@$$(go list -f {{.Version}} -m github.com/brimdata/zed)

.PHONY: build
build: build/dist/zeek build/dist/suricata
	@go build -ldflags='$(LDFLAGS)' -o build/dist ./cmd/...

.PHONY: release
release: build
	@cd build \
		&& mv dist brimcap \
		&& $(ZIP) brimcap-$(VERSION).$$(go env GOOS)-$$(go env GOARCH).zip brimcap \
		&& rm -rf brimcap

.PHONY: vet
vet:
	@go vet ./...

.PHONY: generate
generate:
	@GOBIN="$(CURDIR)/bin" go install github.com/golang/mock/mockgen
	@PATH="$(CURDIR)/bin:$(PATH)" go generate ./...

.PHONY: test
test:
	@go test -timeout 1m ./...

.PHONY: ztest-run
ztest-run: build bin/zq
	@ZTEST_PATH="$(CURDIR)/build/dist:$(CURDIR)/bin:$(PATH)" go test . -run $(TEST)

.PHONY: ztest
ztest: build bin/zq
	@ZTEST_PATH="$(CURDIR)/build/dist:$(CURDIR)/bin:$(PATH)" go test .
