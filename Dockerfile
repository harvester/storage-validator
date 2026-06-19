FROM registry.suse.com/bci/golang:1.26 AS builder

ARG MK_HOST_ARCH
ENV ARCH=$MK_HOST_ARCH
ENV GOTOOLCHAIN=auto
ARG CONTAINER_WORKDIR=/go/src/github.com/harvester/storage-validator

RUN go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2

ENV GO111MODULE=on
ENV HOME=/go/src/github.com/harvester/storage-validator

# ---- base ----
FROM builder AS base
WORKDIR /go/src/github.com/harvester/storage-validator

# to exclude some files, add them in .dockerignore
COPY . .

# ---- build ----
FROM base AS build
ARG MK_REPO_ID

RUN --mount=type=cache,target=/go/pkg/mod,id=storage-validator-go-mod-${MK_REPO_ID} \
    --mount=type=cache,target=/go/src/github.com/harvester/storage-validator/.cache/go-build,id=storage-validator-go-build-${MK_REPO_ID} \
    ./scripts/ci

FROM scratch AS build-output
COPY --from=build /go/src/github.com/harvester/storage-validator/bin/ /bin/