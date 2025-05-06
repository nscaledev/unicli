# Application version encoded in all the binaries.
VERSION = 0.0.0

# Base go module name.
MODULE := $(shell cat go.mod | grep -m1 module | awk '{print $$2}')

# Git revision.
REVISION := $(shell git rev-parse HEAD)

# Commands to build, the first lot are architecture agnostic and will be built
# for your host's architecture.  The latter are going to run in Kubernetes, so
# want to be amd64.
COMMANDS = \
  kubectl-unikorn

# Release will do cross compliation of all images for the 'all' target.
# Note we aren't fucking about with docker here because that opens up a
# whole can of worms to do with caching modules and pisses on performance,
# primarily making me rage.  For image creation, this, by necessity,
# REQUIRES multiarch images to be pushed to a remote registry because
# Docker apparently cannot support this after some 3 years...  So don't
# run that target locally when compiling in release mode.
ifdef RELEASE
COMMAND_TARGETS := amd64-linux arm64-linux arm64-darwin
else
COMMAND_TARGETS := $(shell go env GOARCH)-$(shell go env GOOS)
endif

# Some constants to describe the repository.
BINDIR = bin
CMDDIR = cmd

# Where to install things.
PREFIX = $(HOME)/bin

# List of binaries to build.
COMMAND_BINARIES := $(foreach target,$(COMMAND_TARGETS),$(foreach ctrl,$(COMMANDS),$(BINDIR)/$(target)/$(ctrl)))

# List of sources to trigger a build.
# TODO: Bazel may be quicker, but it's a massive hog, and a pain in the arse.
SOURCES := $(shell find . -type f -name *.go) $(shell find . -type f -name *.tmpl)

# Some bits about go.
GOPATH := $(shell go env GOPATH)
GOBIN := $(if $(shell go env GOBIN),$(shell go env GOBIN),$(GOPATH)/bin)

# Common linker flags.
FLAGS=-trimpath -ldflags '-X $(MODULE)/pkg/constants.Version=$(VERSION) -X $(MODULE)/pkg/constants.Revision=$(REVISION)'

# Defines the linter version.
LINT_VERSION=v2.1.5

# Main target, builds all binaries.
.PHONY: all
all: $(COMMAND_BINARIES)

# Create a binary output directory, this should be an order-only prerequisite.
$(BINDIR) $(BINDIR)/amd64-linux $(BINDIR)/arm64-linux $(BINDIR)/amd64-darwin $(BINDIR)/arm64-darwin:
	mkdir -p $@

# Create a binary from a command.
$(BINDIR)/%: $(SOURCES) $(GENDIR) $(OPENAPI_FILES) | $(BINDIR)
	CGO_ENABLED=0 go build $(FLAGS) -o $@ $(CMDDIR)/$*/main.go

$(BINDIR)/amd64-linux/%: $(SOURCES) $(GENDIR) $(OPENAPI_FILES) | $(BINDIR)/amd64-linux
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(FLAGS) -o $@ $(CMDDIR)/$*/main.go

$(BINDIR)/arm64-linux/%: $(SOURCES) $(GENDIR) | $(BINDIR)/arm64-linux
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(FLAGS) -o $@ $(CMDDIR)/$*/main.go

$(BINDIR)/arm64-darwin/%: $(SOURCES) $(GENDIR) | $(BINDIR)/arm64-darwin
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(FLAGS) -o $@ $(CMDDIR)/$*/main.go

# Build a binary and install it.
$(PREFIX)/%: $(BINDIR)/%
	install -m 750 $< $@

# Perform linting.
# This must pass or you will be denied by CI.
.PHOMY: lint
lint: $(GENDIR)
	@go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
	$(GOBIN)/golangci-lint run ./...

# Perform license checking.
# This must pass or you will be denied by CI.
.PHONY: license
license:
	go run github.com/unikorn-cloud/core/hack/check_license
