#!/bin/bash

export VERSION=20250410
docker buildx build \
	--builder ${BUILDER} \
	--platform linux/amd64 \
	-t open3fs/3fs:${VERSION} \
	--build-arg VERSION=${VERSION} \
	-f Dockerfile .
