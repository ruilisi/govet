GO ?= go

VERSION := v0.1.1
RELEASE_NOTE := "Empty denied imports"

git-tag:
	git tag -a $(VERSION) -m $(RELEASE_NOTE)
	git push github $(VERSION)

release: git-tag
	goreleaser release

.PHONY: build
build:
	$(GO) build

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet: build
	$(GO) vet ./...
	$(GO) vet -vettool=gitea-vet ./...

.PHONY: lint
lint:
	@hash golangci-lint > /dev/null 2>&1; if [ $$? -ne 0 ]; then \
		export BINARY="golangci-lint"; \
		curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh | sh -s -- -b $(shell $(GO) env GOPATH)/bin v1.24.0; \
	fi
	golangci-lint run --timeout 5m
