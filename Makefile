ROOT              := $(realpath $(dir $(realpath $(firstword $(MAKEFILE_LIST)))))

# some systems requires opt-in for buildx
DOCKER_BUILDKIT   := 1
export DOCKER_BUILDKIT

ifdef CI
  BOLD  :=
  CYAN  :=
  RESET :=
else
  BOLD  := \033[1m
  CYAN  := \033[36m
  RESET := \033[0m
endif

BANNER = @printf "$(BOLD)$(CYAN)[target: $@]$(RESET)\n"


# Safely detect a unique system identifier into a variable
MK_SYSTEM_ID := $(strip $(shell \
    if [ -s /etc/machine-id ]; then \
        cat /etc/machine-id 2>/dev/null; \
    elif command -v hostname >/dev/null 2>&1; then \
        hostname 2>/dev/null; \
    else \
        echo -n "unknown"; \
    fi))

# User might have several repos in a host. Distinguish each by using the abs path of the repo
MK_REPO_ID                := $(shell printf '%s' "$(ROOT)$(MK_SYSTEM_ID)" | sha256sum | cut -c1-8)
MK_DOCKER_PROGRESS        ?= plain
MK_DOCKER_PULL            ?= --pull

export MK_DOCKER_PROGRESS MK_DOCKER_PULL MK_REPO_ID

MK_HOST_ARCH ?= $(shell uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
ARCH := $(MK_HOST_ARCH)
export MK_HOST_ARCH
export ARCH

DOCKER_BUILD = docker build $(MK_DOCKER_PULL) \
	--progress=$(MK_DOCKER_PROGRESS) \
	--build-arg MK_REPO_ID \
	--build-arg MK_HOST_ARCH \
	-f $(ROOT)/Dockerfile $(ROOT)

.PHONY: ci


# ---- Directories ----
$(ROOT)/bin:
	@mkdir -p $@

gen-version-env:
	$(BANNER)
	@bash $(ROOT)/scripts/version > /dev/null


# ---- Compile harvester binaries ----
ci: gen-version-env | $(ROOT)/bin
	$(BANNER)
	$(DOCKER_BUILD) --target build-output --output type=local,dest=.

# ---- Clean ----
clean:
	$(BANNER)
	@rm -rf $(ROOT)/bin

.DEFAULT_GOAL := default

default: ci