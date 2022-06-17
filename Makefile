lint:
	golangci-lint run
image:
	./scripts/image.sh
launch:
	./scripts/launch.sh
exe:
	go build l2discovery.go
# Install golangci-lint	
install-lint:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b ${GO_PATH}/bin ${GOLANGCI_VERSION}
