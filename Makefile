generate:
	@git describe --tags --always --match 'v[0-9]*.[0-9]*.[0-9]*' > lib/version/VERSION

all: generate
	CGO_ENABLED=0 go install ./cmd/*
	@cd c; make
	go vet -composites=false ./cmd/*

build-darwin: generate
	(CGO_ENABLED=0 GOOS=darwin go build ./cmd/*)

build-linux: generate
	(CGO_ENABLED=0 GOOS=linux go build ./cmd/*)

build-windows: generate
	(CGO_ENABLED=0 GOOS=windows go build ./cmd/*)

install-darwin: generate
	(CGO_ENABLED=0 GOOS=darwin go install ./cmd/*)

install-linux: generate
	(CGO_ENABLED=0 GOOS=linux go install ./cmd/*)

install-linux-arm: generate
	(CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go install ./cmd/*)

install-windows: generate
	(CGO_ENABLED=0 GOOS=windows go install ./cmd/*)

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
