all: generate
	CGO_ENABLED=0 go install -buildvcs=true ./cmd/*
	@cd c; make
	go vet -composites=false ./cmd/*

build-darwin: generate
	(CGO_ENABLED=0 GOOS=darwin go build -buildvcs=true ./cmd/*)

build-linux: generate
	(CGO_ENABLED=0 GOOS=linux go build -buildvcs=true ./cmd/*)

build-windows: generate
	(CGO_ENABLED=0 GOOS=windows go build -buildvcs=true ./cmd/*)

install-darwin: generate
	(CGO_ENABLED=0 GOOS=darwin go install -buildvcs=true ./cmd/*)

install-linux: generate
	(CGO_ENABLED=0 GOOS=linux go install -buildvcs=true ./cmd/*)

install-linux-arm: generate
	(CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go install -buildvcs=true ./cmd/*)

install-windows: generate
	(CGO_ENABLED=0 GOOS=windows go install -buildvcs=true ./cmd/*)

disruption-manager.tarball: generate
	@./scripts/make-tarball disruption-manager

dominator.tarball: generate
	@./scripts/make-tarball dominator

filegen-server.tarball: generate
	@./scripts/make-tarball filegen-server

fleet-manager.tarball: generate
	@./scripts/make-tarball fleet-manager

hypervisor.tarball: generate
	@./scripts/make-tarball hypervisor init.d/virtual-machines.* \
		-C $(ETCDIR) ssl

image-unpacker.tarball: generate
	@./scripts/make-tarball image-unpacker \
		scripts/image-pusher/export-image

installer.tarball: generate
	@cmd/installer/make-tarball installer

imageserver.tarball: generate
	@./scripts/make-tarball imageserver

imaginator.tarball: generate
	@./scripts/make-tarball imaginator

mdbd.tarball: generate
	@./scripts/make-tarball mdbd

subd.tarball: generate
	@cd c; make
	@./scripts/make-tarball subd           \
		-C cmd/subd  set-owner         \
		-C $(GOPATH) bin/run-in-mntns


UPSTREAM_REPO    := https://github.com/Cloud-Foundations/Dominator.git
BUILD_INFO_FILE  := lib/version/BUILD_INFO
BUILD_INFO_DEPS  := .git/HEAD .git/logs/HEAD .git/refs/tags \
                    $(wildcard .git/packed-refs) Makefile

$(BUILD_INFO_FILE): $(BUILD_INFO_DEPS)
	@version=$$(git describe --tags --always --match 'v[0-9]*.[0-9]*.[0-9]*'); \
	raw_origin=$$(git remote get-url origin 2>/dev/null || echo unknown); \
	origin=$$(echo "$$raw_origin" | sed -e 's|^git@[^:]*:||' -e 's|^https://github.com/||' -e 's|\.git$$||'); \
	branch=$$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo unknown); \
	case "$$raw_origin" in *Cloud-Foundations*) is_fork=false;; *) is_fork=true;; esac; \
	if [ "$$is_fork" = false ] && [ "$$branch" = "master" ]; then \
		behind=0; \
	elif [ -n "$(WITH_UPSTREAM)" ] && \
		git fetch --quiet $(UPSTREAM_REPO) master 2>/dev/null; then \
		behind=$$(git rev-list --count HEAD..FETCH_HEAD 2>/dev/null || echo -1); \
	else \
		behind=-1; \
	fi; \
	{ \
		echo "version=$$version"; \
		echo "origin=$$origin"; \
		echo "branch=$$branch"; \
		echo "behind=$$behind"; \
		echo "fork=$$is_fork"; \
	} > $@.tmp; \
	if cmp -s $@.tmp $@; then rm $@.tmp; else mv $@.tmp $@; fi

build-info: $(BUILD_INFO_FILE)

build-info-upstream:
	@$(MAKE) -B --no-print-directory $(BUILD_INFO_FILE) WITH_UPSTREAM=1

generate: build-info


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
