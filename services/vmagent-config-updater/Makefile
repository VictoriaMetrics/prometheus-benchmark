# All these commands must run from repository root.

DOCKER_NAMESPACE := victoriametrics
GO_BUILDER_IMAGE := golang:1.18.1-alpine

BUILDINFO_TAG ?= $(shell echo $$(git describe --long --all | tr '/' '-')$$( \
	      git diff-index --quiet HEAD -- || echo '-dirty-'$$(git diff-index -u HEAD | openssl sha1 | cut -c 10-17)))

PKG_TAG ?= $(shell git tag -l --points-at HEAD)
ifeq ($(PKG_TAG),)
PKG_TAG := $(BUILDINFO_TAG)
endif

vmagent-config-updater-prod:
	mkdir -p gocache-for-docker
	docker run --rm \
		--user $(shell id -u):$(shell id -g) \
		--mount type=bind,src="$(shell pwd)",dst=/prometheus-benchmark \
		-w /prometheus-benchmark \
		--mount type=bind,src="$(shell pwd)/gocache-for-docker",dst=/gocache \
		--env GOCACHE=/gocache \
		--env GO111MODULE=on \
		--env CGO_ENABLED=0 \
		--env GOOS=linux \
		--env GOARCH=amd64 \
		$(GO_BUILDER_IMAGE) \
		go build -mod=vendor -trimpath -buildvcs=false \
			-ldflags "-s -w -extldflags '-static'" \
			-tags 'netgo osusergo nethttpomithttp2' \
			-o bin/vmagent-config-updater-prod ./services/vmagent-config-updater

vmagent-config-updater-package: vmagent-config-updater-prod
	(docker image ls --format '{{.Repository}}:{{.Tag}}' | grep -q '$(DOCKER_NAMESPACE)/vmagent-config-updater:$(PKG_TAG)$$') || (\
		$(MAKE) vmagent-config-updater-prod && \
		docker build \
			--build-arg src_binary=vmagent-config-updater-prod \
			--tag $(DOCKER_NAMESPACE)/vmagent-config-updater:$(PKG_TAG) \
			-f services/vmagent-config-updater/Dockerfile bin)

vmagent-config-updater-publish: vmagent-config-updater-package
	docker push $(DOCKER_NAMESPACE)/vmagent-config-updater:$(PKG_TAG)

vmagent-config-updater-package-run: vmagent-config-updater-package
	docker run -it --rm \
		--user $(shell id -u):$(shell id -g) \
		--net host \
		$(DOCKER_NAMESPACE)/vmagent-config-updater:$(PKG_TAG)

vmagent-config-updater-run: vmagent-config-updater-prod
	bin/vmagent-config-updater-prod
