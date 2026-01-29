# Version variables - calculated from git tags
# Produces SemVer compliant versions:
#   v1.2.3                                - tagged release (clean)
#   v1.2.3-dirty                          - tagged release with uncommitted changes
#   v1.2.3-dev.5+abc1234                  - 5 commits after tag on main/master
#   v1.2.3-feature-api.5+abc1234          - 5 commits after tag on feature branch
#   v1.2.3-feature-api.5+abc1234-dirty    - feature branch with uncommitted changes
GIT_COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_BRANCH  ?= $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")
BUILD_DATE  ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_DIRTY   := $(shell git diff --quiet 2>/dev/null || echo "-dirty")

# Calculate SemVer compliant version from git tags
GIT_TAG     := $(shell git describe --tags --abbrev=0 2>/dev/null)
COMMITS_SINCE_TAG := $(shell git rev-list $(GIT_TAG)..HEAD --count 2>/dev/null || echo "0")

# Determine if on main/master branch
IS_MAIN_BRANCH := $(filter $(GIT_BRANCH),main master)

# Sanitize branch name for SemVer (replace / and _ with -)
BRANCH_SANITIZED := $(subst /,-,$(subst _,-,$(GIT_BRANCH)))

ifeq ($(GIT_TAG),)
  # No tags exist - use dev
  VERSION ?= dev+$(GIT_COMMIT)$(GIT_DIRTY)
else ifeq ($(COMMITS_SINCE_TAG),0)
  # Exactly on a tag
  VERSION ?= $(GIT_TAG)$(GIT_DIRTY)
else ifneq ($(IS_MAIN_BRANCH),)
  # On main/master, commits after tag: v1.2.3-dev.N+commit
  VERSION ?= $(GIT_TAG)-dev.$(COMMITS_SINCE_TAG)+$(GIT_COMMIT)$(GIT_DIRTY)
else
  # On feature branch: v1.2.3-branch-name.N+commit
  VERSION ?= $(GIT_TAG)-$(BRANCH_SANITIZED).$(COMMITS_SINCE_TAG)+$(GIT_COMMIT)$(GIT_DIRTY)
endif

# Package path for version injection
VERSION_PKG := github.com/Cloud-Foundations/Dominator/lib/version

# Build flags for version injection
LDFLAGS := -X $(VERSION_PKG).Version=$(VERSION)
LDFLAGS += -X $(VERSION_PKG).GitCommit=$(GIT_COMMIT)
LDFLAGS += -X $(VERSION_PKG).GitBranch=$(GIT_BRANCH)
LDFLAGS += -X $(VERSION_PKG).BuildDate=$(BUILD_DATE)

# Output directory for builds
DIST_DIR := dist

# Auto-detect OS for build target
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
  BUILD_OS := linux
else ifeq ($(UNAME_S),Darwin)
  BUILD_OS := darwin
else
  BUILD_OS := linux
endif

# Auto-detect architecture for build target
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
  BUILD_ARCH_SUFFIX :=
else ifeq ($(UNAME_M),aarch64)
  BUILD_ARCH_SUFFIX := -arm
else ifeq ($(UNAME_M),arm64)
  BUILD_ARCH_SUFFIX := -arm
else
  BUILD_ARCH_SUFFIX :=
endif

all:
	CGO_ENABLED=0 go install -ldflags "$(LDFLAGS)" ./cmd/*
	@cd c; make
	go vet -composites=false ./cmd/*

build: build-$(BUILD_OS)$(BUILD_ARCH_SUFFIX)

# Darwin builds exclude Linux-only commands
DARWIN_CMDS := $(filter-out ./cmd/installer ./cmd/subd ./cmd/image-unpacker ./cmd/imaginator,$(wildcard ./cmd/*))

build-darwin:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/ $(DARWIN_CMDS)

build-darwin-arm:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/ $(DARWIN_CMDS)

build-linux:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/ ./cmd/*

build-linux-arm:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/ ./cmd/*

build-windows:
	@mkdir -p $(DIST_DIR)
	CGO_ENABLED=0 GOOS=windows go build -ldflags "$(LDFLAGS)" -o $(DIST_DIR)/ ./cmd/*

clean:
	rm -rf $(DIST_DIR)

install-darwin:
	CGO_ENABLED=0 GOOS=darwin go install -ldflags "$(LDFLAGS)" ./cmd/*

install-darwin-arm:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go install -ldflags "$(LDFLAGS)" ./cmd/*

install-linux:
	CGO_ENABLED=0 GOOS=linux go install -ldflags "$(LDFLAGS)" ./cmd/*

install-linux-arm:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go install -ldflags "$(LDFLAGS)" ./cmd/*

install-windows:
	CGO_ENABLED=0 GOOS=windows go install -ldflags "$(LDFLAGS)" ./cmd/*

# Print version info (useful for debugging)
.PHONY: version-info
version-info:
	@echo "VERSION:    $(VERSION)"
	@echo "GIT_COMMIT: $(GIT_COMMIT)"
	@echo "GIT_BRANCH: $(GIT_BRANCH)"
	@echo "BUILD_DATE: $(BUILD_DATE)"

disruption-manager.tarball:
	@./scripts/make-tarball disruption-manager -C $(ETCDIR) ssl

dominator.tarball:
	@./scripts/make-tarball dominator -C $(ETCDIR) ssl

filegen-server.tarball:
	@./scripts/make-tarball filegen-server -C $(ETCDIR) ssl

fleet-manager.tarball:
	@./scripts/make-tarball fleet-manager -C $(ETCDIR) ssl

hypervisor.tarball:
	@./scripts/make-tarball hypervisor init.d/virtual-machines.* \
		-C $(ETCDIR) ssl

image-unpacker.tarball:
	@./scripts/make-tarball image-unpacker \
		scripts/image-pusher/export-image -C $(ETCDIR) ssl

installer.tarball:
	@cmd/installer/make-tarball installer -C $(ETCDIR) ssl

imageserver.tarball:
	@./scripts/make-tarball imageserver -C $(ETCDIR) ssl

imaginator.tarball:
	@./scripts/make-tarball imaginator -C $(ETCDIR) ssl

mdbd.tarball:
	@./scripts/make-tarball mdbd -C $(ETCDIR) ssl

subd.tarball:
	@cd c; make
	@./scripts/make-tarball subd           \
		-C cmd/subd  set-owner         \
		-C $(GOPATH) bin/run-in-mntns  \
		-C $(ETCDIR) ssl


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
