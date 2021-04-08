VERSION = $(shell git describe --tags --dirty --always)
LDFLAGS = -s -X github.com/brimdata/brimcap/cli.Version=$(VERSION)
ZED_VERSION := $(shell go list -f {{.Version}} -m github.com/brimdata/zed)

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

.PHONY: build
build:
	@mkdir -p dist
	@go build -ldflags='$(LDFLAGS)' -o dist ./cmd/...

bin/zed-$(ZED_VERSION):
	@rm -rf $@*
	@mkdir -p $(@D)
	@echo 'module deps' > $@.mod
	@go get -d -modfile=$@.mod github.com/brimdata/zed@$(ZED_VERSION)
	@go mod download -modfile=$@.mod
	@go build -modfile=$@.mod -o $@ github.com/brimdata/zed/cmd/zed

.PHONY: bin/zed
bin/zed: bin/zed-$(ZED_VERSION)
	@ln -fs $(<F) $@

.PHONY: vet
vet:
	@go vet ./...

.PHONY: generate
generate:
	@GOBIN="$(CURDIR)/bin" go install github.com/golang/mock/mockgen
	@PATH="$(CURDIR)/bin:$(PATH)" go generate ./...

.PHONY: test
test:
	go test ./...

.PHONY: exists-%
exists-%:
	@hash $* 2>/dev/null \
		|| { echo >&2 "command '$*' required but is not installed" ; exit 1; }

.PHONY: ztest-run
ztest-run: build bin/zed exists-zeek exists-suricata
	@ZTEST_PATH="$(CURDIR)/dist:$(CURDIR)/bin:$(PATH)" go test . -run $(TEST)

.PHONY: ztest
ztest: build bin/zed exists-zeek exists-suricata
	@ZTEST_PATH="$(CURDIR)/dist:$(CURDIR)/bin:$(PATH)" go test .

.PHONY: install
install:
	@go install -ldflags='$(LDFLAGS)' ./cmd/... 
