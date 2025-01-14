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

install-windows:
	(CGO_ENABLED=0 GOOS=windows go install ./cmd/*)

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
