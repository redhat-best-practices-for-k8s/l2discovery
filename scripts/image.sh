#!/bin/sh
set -x
VERSION=latest
IMAGE_TAG=l2discovery
REPO=quay.io/redhat-cne
make test
go build l2discovery.go
podman build -t ${IMAGE_TAG} --rm -f Dockerfile .
podman tag ${IMAGE_TAG} ${REPO}/${IMAGE_TAG}:${VERSION}
podman push ${REPO}/${IMAGE_TAG}:${VERSION}
