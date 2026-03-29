all:
	CGO_ENABLED=0 go install ./cmd/*
	@cd c; make
	go vet -composites=false ./cmd/*

build-darwin:
	(CGO_ENABLED=0 GOOS=darwin go build ./cmd/*)

build-linux:
	(CGO_ENABLED=0 GOOS=linux go build ./cmd/*)

build-windows:
	(CGO_ENABLED=0 GOOS=windows go build ./cmd/*)

install-darwin:
	(CGO_ENABLED=0 GOOS=darwin go install ./cmd/*)

install-linux:
	(CGO_ENABLED=0 GOOS=linux go install ./cmd/*)

install-linux-arm:
	(CGO_ENABLED=0 GOARCH=arm64 GOOS=linux go install ./cmd/*)

install-windows:
	(CGO_ENABLED=0 GOOS=windows go install ./cmd/*)

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
