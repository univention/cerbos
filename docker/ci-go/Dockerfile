# SPDX-FileCopyrightText: 2026 Univention GmbH
# SPDX-License-Identifier: AGPL-3.0-only

# Go toolchain plus the module cache and a warm build cache for build-cerbos.
# Keep the base in sync with GO_VERSION and GO_IMAGE_DIGEST in .gitlab-ci.yml; a change here rebuilds the image.
ARG DOCKER_PROXY
FROM ${DOCKER_PROXY}golang:1.27.4@sha256:5e8afd493a7124a8586e134d9b532731161d4b9becf9dee5a5956b107426f93d

ENV GOMODCACHE=/opt/go/mod \
    GOCACHE=/opt/go/build \
    GOFLAGS=-modcacherw

# Builds without -trimpath key their cache entries on the source path, so compile where CI checks out.
ARG PROJECT_DIR=/builds/univention/dev/projects/authorization-engine/cerbos
WORKDIR ${PROJECT_DIR}

# The bind mount keeps the sources and .git out of the image.
RUN --mount=type=bind,target=.,rw \
    go list -deps -test -tags=tests ./... > /dev/null \
    && GOWORK=off go mod download -C hack/tools/testsplit \
    && CGO_ENABLED=0 GOAMD64=v1 go build -trimpath -buildvcs=false -o /dev/null ./cmd/cerbos

WORKDIR /
