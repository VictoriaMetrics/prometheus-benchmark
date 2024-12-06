# All these commands must run from repository root.

SERVICES_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

DOCKER_NAMESPACE := victoriametrics
GO_BUILDER_IMAGE := golang:1.23.4-alpine

BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -c 10-17)))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

%-exists:
	$(eval SERVICE := $*)
	if [ ! -d $(SERVICES_DIR)/$(SERVICE) ]; then \
		echo "Service $(SERVICE) does not exist"; \
		exit 1; \
	fi

%-go-build: %-exists
	$(eval SERVICE := $*)
	mkdir -p gocache-for-docker
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(SERVICES_DIR)/$(SERVICE)",dst=/config-updater \
		-w /config-updater \
		--mount type=bind,src="$(SERVICES_DIR)/$(SERVICE)/gocache-for-docker",dst=/gocache \
		--env GOCACHE=/gocache \
		--env GO111MODULE=on \
		--env CGO_ENABLED=0 \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		$(GO_BUILDER_IMAGE) \
		go build -mod=vendor -trimpath -buildvcs=false \
			-ldflags "-s -w -extldflags '-static'" \
			-tags 'netgo osusergo nethttpomithttp2' \
			-o bin/config-updater .

%-go-package: %-go-build
	$(eval SERVICE := $*)
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(DOCKER_NAMESPACE)/$(SERVICE):$(PKG_TAG)$$') || (\
		docker build \
			--tag $(DOCKER_NAMESPACE)/$(SERVICE):$(PKG_TAG) \
			$(SERVICES_DIR)/$(SERVICE))

%-go-publish: %-go-package
	$(eval SERVICE := $*)
	docker push $(DOCKER_NAMESPACE)/$(SERVICE):$(PKG_TAG)

%-go-package-run: %-go-package
	$(eval SERVICE := $*)
	docker run -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_NAMESPACE)/$(SERVICE):$(PKG_TAG)

%-go-run: %-exists
	$(eval SERVICE := $*)
	cd $(SERVICES_DIR)/$(SERVICE) && go run .
