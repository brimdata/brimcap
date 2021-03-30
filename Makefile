# This enables a shortcut to run a single test from the ./ztests suite, e.g.:
#  make TEST=TestZq/ztests/suite/cut/cut
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

.PHONY: zq
zq:
	@go mod download
	@GOBIN="$(CURDIR)/bin" go install github.com/brimsec/zq/cmd/zq

.PHONY: vet
vet:
	@go vet -composites=false -stdmethods=false ./...

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
ztest-run: build zq exists-zeek exists-suricata
	@zeek=$$(dirname $$(which zeek)) ; suricata=$$(dirname $$(which suricata)) ; \
		ZTEST_PATH="$(CURDIR)/dist:$(CURDIR)/bin:$${zeek}:$${suricata}" go test . -run $(TEST)

.PHONY: ztest
ztest: build zq exists-zeek exists-suricata
	@zeek=$$(dirname $$(which zeek)) ; suricata=$$(dirname $$(which suricata)) ; \
		ZTEST_PATH="$(CURDIR)/dist:$(CURDIR)/bin:$${zeek}:$${suricata}" go test .


.PHONY: install
install:
	@go install -ldflags='$(LDFLAGS)' ./cmd/... 
