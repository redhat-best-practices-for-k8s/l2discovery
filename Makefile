GO_PACKAGES=$(shell go list ./... | grep -v vendor)

# Get default value of $GOBIN if not explicitly set
GO_PATH=$(shell go env GOPATH)
ifeq (,$(shell go env GOBIN))
  GOBIN=${GO_PATH}/bin
else
  GOBIN=$(shell go env GOBIN)
endif

# Variables
GOLANGCI_VERSION=v1.49.0

lint:
	golangci-lint run --timeout 10m0s
image:
	./scripts/image.sh
launch:
	./scripts/launch.sh
build:
	go build l2discovery.go
# Install golangci-lint	
install-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GO_PATH}/bin ${GOLANGCI_VERSION}
vet:
	go vet ${GO_PACKAGES}
build-all:
	$(MAKE) build -C .
	$(MAKE) build -C ./apps/l2dump
lint-all:
	$(MAKE) lint -C .
	$(MAKE) lint -C ./apps/l2dump
	$(MAKE) lint -C ./l2lib
	$(MAKE) lint -C ./l2lib/pkg/export
