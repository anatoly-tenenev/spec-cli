BINARY := spec-cli
CMD := ./cmd/spec-cli
DIST_DIR ?= dist
VERSION ?= dev
LDFLAGS ?= -X github.com/anatoly-tenenev/spec-cli/internal/buildinfo.Version=$(VERSION)
RELEASE_TARGETS := \
	linux:amd64:tar.gz \
	linux:arm64:tar.gz \
	darwin:amd64:tar.gz \
	darwin:arm64:tar.gz \
	windows:amd64:zip \
	windows:arm64:zip

.PHONY: fmt vet test build run release release-verify-version clean-dist release-build release-checksums

fmt:
	go fmt ./...

vet:
	go vet ./...

test:
	go test ./...

build:
	mkdir -p bin
	go build -ldflags "$(LDFLAGS)" -o bin/$(BINARY) $(CMD)

run:
	go run $(CMD) $(ARGS)

release: release-verify-version clean-dist release-build release-checksums

release-verify-version:
	@if [ -z "$(VERSION)" ] || [ "$(VERSION)" = "dev" ]; then \
		echo "VERSION must be set to a release version, e.g. make release VERSION=0.1.0" >&2; \
		exit 2; \
	fi

clean-dist:
	rm -rf $(DIST_DIR)
	mkdir -p $(DIST_DIR)

release-build:
	@set -eu; \
	if ! command -v zip >/dev/null 2>&1; then \
		echo "zip is required to package Windows release archives" >&2; \
		exit 2; \
	fi; \
	for target in $(RELEASE_TARGETS); do \
		goos=$${target%%:*}; \
		rest=$${target#*:}; \
		goarch=$${rest%%:*}; \
		format=$${rest#*:}; \
		name="$(BINARY)_$(VERSION)_$${goos}_$${goarch}"; \
		stage="$(DIST_DIR)/$$name"; \
		bin_name="$(BINARY)"; \
		if [ "$$goos" = "windows" ]; then \
			bin_name="$(BINARY).exe"; \
		fi; \
		mkdir -p "$$stage"; \
		CGO_ENABLED=0 GOOS="$$goos" GOARCH="$$goarch" \
			go build -ldflags "$(LDFLAGS)" -o "$$stage/$$bin_name" $(CMD); \
		if [ "$$format" = "zip" ]; then \
			( cd "$(DIST_DIR)" && zip -rq "$$name.zip" "$$name" ); \
		else \
			tar -C "$(DIST_DIR)" -czf "$(DIST_DIR)/$$name.tar.gz" "$$name"; \
		fi; \
		rm -rf "$$stage"; \
	done

release-checksums:
	@set -eu; \
	cd "$(DIST_DIR)"; \
	if command -v shasum >/dev/null 2>&1; then \
		shasum -a 256 * > checksums.txt; \
	elif command -v sha256sum >/dev/null 2>&1; then \
		sha256sum * > checksums.txt; \
	else \
		echo "checksum tool is required: shasum or sha256sum" >&2; \
		exit 2; \
	fi
