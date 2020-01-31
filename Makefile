all:
	@go install ./cmd/...
	@cd c; make

build-darwin:
	mkdir -p bins
	cd bins; for name in ../cmd/*; do GOOS=darwin go build $$name; done

build-linux:
	mkdir -p bins
	cd bins; for name in ../cmd/*; do GOOS=linux go build $$name; done

build-windows:
	mkdir -p bins
	cd bins; for name in ../cmd/*; do GOOS=windows go build $$name; done

install-darwin:
	@GOOS=darwin go install ./cmd/...

install-linux:
	@GOOS=linux go install ./cmd/...

install-windows:
	@GOOS=windows go install ./cmd/...

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
	@./scripts/make-tarball mdbd

subd.tarball:
	@cd c; make
	@./scripts/make-tarball subd -C $(GOPATH) bin/run-in-mntns \
		-C $(ETCDIR) ssl


format:
	gofmt -s -w .

format-imports:
	goimports -w .


test:
	@find * -name '*_test.go' |\
	sed -e 's@^@github.com/Cloud-Foundations/Dominator/@' -e 's@/[^/]*$$@@' |\
	sort -u | xargs go test
