all:
	CGO_ENABLED=0 go install $(GO_BUILD_FLAGS) ./cmd/*
	@cd c; make
	go vet -composites=false ./cmd/*

build-darwin:
	(CGO_ENABLED=0 GOOS=darwin go build $(GO_BUILD_FLAGS) ./cmd/*)

build-linux:
	(CGO_ENABLED=0 GOOS=linux go build $(GO_BUILD_FLAGS) ./cmd/*)

build-windows:
	(CGO_ENABLED=0 GOOS=windows go build $(GO_BUILD_FLAGS) ./cmd/*)

install-darwin:
	(CGO_ENABLED=0 GOOS=darwin go install $(GO_BUILD_FLAGS) ./cmd/*)

install-linux:
	(CGO_ENABLED=0 GOOS=linux go install $(GO_BUILD_FLAGS) ./cmd/*)

install-linux-arm:
	(CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go install $(GO_BUILD_FLAGS) ./cmd/*)

install-windows:
	(CGO_ENABLED=0 GOOS=windows go install $(GO_BUILD_FLAGS) ./cmd/*)

disruption-manager.tarball:
	@./scripts/make-tarball disruption-manager

dominator.tarball:
	@./scripts/make-tarball dominator

filegen-server.tarball:
	@./scripts/make-tarball filegen-server

fleet-manager.tarball:
	@./scripts/make-tarball fleet-manager

hypervisor.tarball:
	@./scripts/make-tarball hypervisor init.d/virtual-machines.* \
		-C $(ETCDIR) ssl

image-unpacker.tarball:
	@./scripts/make-tarball image-unpacker \
		scripts/image-pusher/export-image

installer.tarball:
	@cmd/installer/make-tarball installer

imageserver.tarball:
	@./scripts/make-tarball imageserver

imaginator.tarball:
	@./scripts/make-tarball imaginator

mdbd.tarball:
	@./scripts/make-tarball mdbd

subd.tarball:
	@cd c; make
	@./scripts/make-tarball subd           \
		-C cmd/subd  set-owner         \
		-C $(GOPATH) bin/run-in-mntns


UPSTREAM_REPO  := https://github.com/Cloud-Foundations/Dominator.git
VERSION_PKG    := github.com/Cloud-Foundations/Dominator/lib/version
RAW_ORIGIN     := $(shell git remote get-url origin 2>/dev/null || echo unknown)
GIT_VERSION    := $(shell git describe --tags --always \
                    --match 'v[0-9]*.[0-9]*.[0-9]*' 2>/dev/null || echo unknown)
GIT_ORIGIN     := $(shell echo '$(RAW_ORIGIN)' | sed -e 's|^git@[^:]*:||' \
                    -e 's|^https://[^@/]*@|https://|' \
                    -e 's|^https://github.com/||' -e 's|\.git$$||')
GIT_BRANCH     := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || \
                    echo unknown)
GIT_FORK       := $(if $(findstring Cloud-Foundations,$(RAW_ORIGIN)),false,true)
GIT_BEHIND     := $(shell \
	if [ "$(GIT_FORK)" = false ] && [ "$(GIT_BRANCH)" = master ]; then \
		echo 0; \
	elif [ -n "$(WITH_UPSTREAM)" ] && \
		git fetch --quiet $(UPSTREAM_REPO) master 2>/dev/null; then \
		git rev-list --count HEAD..FETCH_HEAD 2>/dev/null || echo -1; \
	else \
		echo -1; \
	fi)

export GO_LDFLAGS := -X $(VERSION_PKG).buildVersion=$(GIT_VERSION) \
                     -X $(VERSION_PKG).buildOrigin=$(GIT_ORIGIN) \
                     -X $(VERSION_PKG).buildBranch=$(GIT_BRANCH) \
                     -X $(VERSION_PKG).buildBehind=$(GIT_BEHIND) \
                     -X $(VERSION_PKG).buildFork=$(GIT_FORK)
GO_BUILD_FLAGS := -buildvcs=true -ldflags "$(GO_LDFLAGS)"


format:
	gofmt -s -w .

format-imports:
	goimports -w .


test:
	@find * -name '*_test.go' |\
	sed -e 's@^@github.com/Cloud-Foundations/Dominator/@' -e 's@/[^/]*$$@@' |\
	sort -u | xargs go test


vet:
	go vet ./cmd/*

website:
	@./scripts/make-website ${TARGET_DIR}
